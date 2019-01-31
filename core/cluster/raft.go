package cluster

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/emirpasic/gods/sets/hashset"
	pb "github.com/infinitbyte/framework/core/cluster/pb"
	"github.com/infinitbyte/framework/core/cluster/raft"
	"github.com/infinitbyte/framework/core/cluster/raft-boltdb"
	"github.com/infinitbyte/framework/core/env"
	"github.com/infinitbyte/framework/core/errors"
	"github.com/infinitbyte/framework/core/global"
	"github.com/infinitbyte/framework/core/rpc"
	"github.com/infinitbyte/framework/core/util"
	"io/ioutil"
	"net"
	"os"
	"path"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

const (
	retainSnapshotCount = 2
	raftTimeout         = 10 * time.Second
)

type RaftModule struct {
	tcpAddr   *net.TCPAddr
	addr      string
	cfg       *raft.Config
	raft      *raft.Raft
	transport *raft.NetworkTransport
	fsm       *ClusterFSM
	localNode *Node
}

var raftModule *RaftModule

func New() *RaftModule {
	cfg := raft.DefaultConfig()
	raftModule = &RaftModule{
		cfg: cfg,
		fsm: NewFSM(),
	}

	InitAPI()

	return raftModule
}

func getRaft() *RaftModule {
	if raftModule == nil {
		panic(errors.New("raft not ready"))
	}
	return raftModule
}

func GetLeader() string {
	return getRaft().raft.Leader()
}

func GetPeers() []string {
	return getRaft().raft.Peers()
}

func GetClusterReadonlyMetadata() Metadata {
	return getRaft().fsm.GetClusterMetadata()
}

func GetLocalActivePeersCount() int {
	count := 0
	for _, v := range localKnowPeers {
		if v.Active {
			count++
		}
	}
	return count
}

func GetLocalPeers() map[string]*Node {
	return localKnowPeers
}

func GetActivePeers() ([]string, map[string]*Node) {
	out := map[string]*Node{}
	p := []string{}
	for k, v := range localKnowPeers {
		if v.Active {
			out[k] = v
			p = append(p, k)
		}
	}
	return p, out
}

func GetRaftStatus() string {
	if getRaft().raft.Leader() == "" {
		return "red"
	} else {
		return "green"
	}
}

func GetStats() map[string]string {
	return getRaft().raft.Stats()
}

type NullWriter int

func (NullWriter) Write([]byte) (int, error) { return 0, nil }

func Open() (err error) {

	getRaft().addr = util.GetSafetyInternalAddress(global.Env().SystemConfig.NetworkConfig.RPCBinding)

	getRaft().tcpAddr, err = net.ResolveTCPAddr("tcp", getRaft().addr)
	if err != nil {
		panic(err)
	}

	//wait to get enough peers
	signal := make(chan bool)
	go ServeMulticastDiscovery(getRaft().multicastCallback, signal)
	<-signal

	var seeds []string
	if len(global.Env().SystemConfig.ClusterConfig.Seeds) > 0 {
		log.Tracef("use config seeds")
		seeds = global.Env().SystemConfig.ClusterConfig.Seeds
	}

	if GetLocalActivePeersCount() > 1 && GetLocalActivePeersCount() >= global.Env().SystemConfig.ClusterConfig.MinimumNodes {
		seeds, _ = GetActivePeers()
		log.Tracef("prefer use local active peers as seeds, %v", len(seeds))
	}

	err = getRaft().restart(seeds)
	if err != nil {
		log.Error(err)
		panic(err)
	}

	//Hello world, I am up, where is the cluster?
	discoveryCluster()

	//How are you, I'll reach to you
	runTasks()

	registerNode(getRaft().localNode)

	return err
}

func discoveryCluster() {

	req := Request{}

	node :=
		Node{
			APIEndpoint: util.GetSafetyInternalAddress(global.Env().SystemConfig.NetworkConfig.APIBinding),
			RPCEndpoint: util.GetSafetyInternalAddress(global.Env().SystemConfig.NetworkConfig.RPCBinding),
			ClusterName: global.Env().SystemConfig.ClusterConfig.Name,
			StartTime:   env.GetStartTime().Unix(),
			Name:        global.Env().SystemConfig.NodeConfig.Name}

	req.Node = node
	req.NodeType = EVENT_LEADER_DISCOVERY
	getRaft().localNode = &node
	Broadcast(&req)

}

func runTasks() {

	//if listen as leader, start heartbeat service
	go func() {

	start_loop:

		for {
			if getRaft().raft == nil || getRaft().raft.Leader() != "" && getRaft().raft.Leader() != getRaft().addr {
				log.Trace("I am not qualified to run the health check, skip")
				goto quit_loop
			}

			if getRaft().raft.Leader() == "" {
				if GetLocalActivePeersCount() > 0 {
					//try nominate
					nominate()
				} else {
					//where is cluster
					discoveryCluster()
				}

				goto quit_loop
				continue
			} else {
				//leader check all the connections
				runCheck()
			}

			time.Sleep(time.Second * 5)
		}

	quit_loop:
		log.Trace("pause heartbeat service")
		time.Sleep(time.Second * 30)
		goto start_loop
	}()
}

func runCheck() {

	log.Tracef("start heartbeat check, %v hosts", len(localKnowPeers))
	localRpc := util.GetSafetyInternalAddress(global.Env().SystemConfig.NetworkConfig.RPCBinding)

	for i, v := range localKnowPeers {
		log.Trace("heartbeat:", i, ",", v, ",", v.RPCEndpoint, " vs ", localRpc)

		previousState := v.Active

		////ignore local node
		if v.RPCEndpoint == localRpc {

			//if I am leader and I am not in the list, add it
			if v, ok := localKnowPeers[v.RPCEndpoint]; !ok && !v.Active {
				log.Error("adding leader myself")
				v.Active = true
				getRaft().Up(v)
			}

			log.Trace("ignore self check")
			continue
		}

		conn, err := rpc.ObtainConnection(v.RPCEndpoint)
		if err != nil {
			v.Active = false
			getRaft().Leave(v.RPCEndpoint, v.RPCEndpoint)
		}

		defer conn.Close()
		c := pb.NewDiscoveryClient(conn)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		r, err := c.Ping(ctx, &pb.HealthCheckRequest{
			NodeName: global.Env().SystemConfig.NodeConfig.Name,
			NodeIp:   getRaft().tcpAddr.IP.String(),
			NodePort: uint32(getRaft().tcpAddr.Port),
		})

		if r == nil || (err != nil && previousState == true) {
			log.Error(v.RPCEndpoint, ",", err)
			v.Active = false
			getRaft().Down(v.RPCEndpoint, v.RPCEndpoint)
			continue
		}

		if r.Success && !previousState {
			v.Active = true
			getRaft().Up(v)
		}
		log.Trace("heartbeat success,", r.Success)

	}
}

var restartLock sync.Mutex

func (s *RaftModule) restart(seeds []string) (err error) {
	restartLock.Lock()
	defer restartLock.Unlock()

	if time.Now().Sub(electedTime).Seconds() < 30 {
		log.Debug("skip election, already in progress")
		return
	}

	log.Trace("restart with seeds,", seeds)

	if getRaft().raft != nil && getRaft().raft.State() != raft.Shutdown {
		err = getRaft().raft.Shutdown().Error()
		if err != nil {
			log.Error(err)
			return err
		}

	}

	if global.Env().SystemConfig.ClusterConfig.MinimumNodes == 1 && len(seeds) == 0 {
		log.Warnf("minimum raft number is 1 and got zero seeds, will start as leader")
		s.cfg.StartAsLeader = true
	}

	addr, err := net.ResolveTCPAddr("tcp", s.addr)
	if err != nil {
		log.Error(err)
		return err
	}

	s.transport, err = raft.NewTCPTransport(s.addr, addr, 3, 10*time.Second, os.Stderr, rpc.GetListener())
	if err != nil {
		log.Error(err)
		return err
	}

	if !global.Env().IsDebug {
		//disable raft logging
		s.cfg.LogOutput = new(NullWriter)
	}

	peerStore := &raft.StaticPeers{StaticPeers: seeds}

	dir := path.Join(global.Env().GetWorkingDir(), "raft")
	// Create the snapshot store. This allows the Raft to truncate the log.
	snapshots, err := raft.NewFileSnapshotStore(dir, retainSnapshotCount, os.Stderr)
	if err != nil {
		return fmt.Errorf("file snapshot store: %s", err)
	}

	// Create the log store and stable store.
	logStore, err := raftboltdb.NewBoltStore(filepath.Join(dir, "raft.db"))

	if err != nil {
		panic(err)
	}

	// Instantiate the Raft systems.
	ra, err := raft.NewRaft(s.cfg, s.fsm, logStore, logStore, snapshots, peerStore, s.addr, s.transport)
	if err != nil {
		return fmt.Errorf("new raft: %s", err)
	}
	getRaft().raft = ra

	log.Trace("raft initialized")

	//started = true
	electedTime = time.Now()
	return nil
}

var localKnowPeers = make(map[string]*Node)
var ticketsBox = hashset.New()

const EVENT_LEADER_DISCOVERY = "LEADER_DISCOVERY"
const EVENT_LEADER_ECHO = "LEADER_ECHO"
const EVENT_LEADER_NOMINATE = "LEADER_NOMINATE"
const EVENT_LEADER_READY = "LEADER_READY"

var electedTime time.Time

func (s *RaftModule) multicastCallback(src *net.UDPAddr, n int, b []byte) {

	v := Request{}

	util.FromJSONBytes(b[:n], &v)

	log.Tracef("received multicast message: %v", v)

	if getRaft().raft == nil {
		log.Error("raft is not initialized")
		return
	}

	//node is active
	v.Node.Active = true
	registerNode(&v.Node)

	switch v.NodeType {

	case EVENT_LEADER_DISCOVERY:

		if getRaft().raft.Leader() == getRaft().addr {
			registerNode(&v.Node)

			req := Request{}
			req.Node = v.Node
			req.FromNode = *getRaft().localNode
			req.NodeType = EVENT_LEADER_ECHO

			Broadcast(&req)

			getRaft().raft.AddPeer(v.Node.RPCEndpoint)
		}

		break
	case EVENT_LEADER_ECHO:

		if v.Node.RPCEndpoint == getRaft().addr && getRaft().raft.Leader() == "" {
			registerNode(&v.FromNode)
			err := getRaft().restart([]string{v.FromNode.RPCEndpoint})
			if err != nil {
				panic(err)
			}
			return
		}

		break
	case EVENT_LEADER_READY:
		log.Tracef("leader %v is ready", v.Node.RPCEndpoint)
		if getRaft().raft.Leader() == "" {
			seeds, _ := GetActivePeers()
			err := getRaft().restart(seeds)
			if err != nil {
				panic(err)
			}
		}

		break
	case EVENT_LEADER_NOMINATE:

		l.Lock()
		defer l.Unlock()

		log.Tracef("check nominate message, %v vs %v ", v.Node.RPCEndpoint, s.addr)

		if getRaft().raft.Leader() == s.addr && getRaft().raft.Leader() != "" {

			getRaft().raft.AddPeer(v.Node.RPCEndpoint)

			registerNode(&v.FromNode)

			req := Request{}
			req.Node = v.Node
			req.FromNode = *getRaft().localNode
			req.NodeType = EVENT_LEADER_ECHO

			Broadcast(&req)
			return
		}

		//if the nominate is for self, tickets +1
		if v.Node.RPCEndpoint != getRaft().addr && v.Node.RPCEndpoint == s.addr && getRaft().raft.Leader() == "" {
			ticketsBox.Add(v.FromNode.RPCEndpoint)
			log.Debugf("i am nominated as leader, got ticket: %v ,from:%s", ticketsBox.Size(), src.String())

			//if get quorum tickets, start to runLeader
			if ticketsBox.Size() >= global.Env().SystemConfig.ClusterConfig.MinimumNodes &&
				ticketsBox.Size() >= ((GetLocalActivePeersCount()+1)/2) &&
				GetLocalActivePeersCount() > 0 &&
				GetLocalActivePeersCount() >= global.Env().SystemConfig.ClusterConfig.MinimumNodes {

				log.Debug("got enough tickets, restart as leader now")
				seeds := []string{}
				for _, v := range ticketsBox.Values() {
					seeds = append(seeds, v.(string))
				}

				err := s.restart(seeds)
				if err != nil {
					panic(err)
					log.Error(err)
				}

				ticketsBox.Clear()

				req := Request{}
				req.Node = *getRaft().localNode
				req.NodeType = EVENT_LEADER_READY
				Broadcast(&req)

			}
		}

		break
	}
}

//register local node status
func registerNode(node *Node) {
	if add, ok := localKnowPeers[node.RPCEndpoint]; !ok {

		node.Active = true
		if node.ClusterName != global.Env().SystemConfig.ClusterConfig.Name {
			log.Error("cluster name mismatch, ignore node,", global.Env().SystemConfig.ClusterConfig.Name, " vs ", node.ClusterName)
			return
		}

		log.Tracef("add %s to know peers", node.RPCEndpoint)

		localKnowPeers[node.RPCEndpoint] = node

		if getRaft().raft.State() == raft.Leader {
			//auto join new nodes
			getRaft().Up(node)
			log.Debug("add new node to cluster,", node.RPCEndpoint)
		}
	} else {
		add.Active = true
	}
}

var lastNomiateTime time.Time
var lastNomiateNode string

func nominate() {

	if getRaft().raft.Leader() == "" && GetLocalActivePeersCount() > 0 && GetLocalActivePeersCount() >= global.Env().SystemConfig.ClusterConfig.MinimumNodes {
		log.Debugf("met enough raft nodes(%v), start nomination", global.Env().SystemConfig.ClusterConfig.MinimumNodes)

		//check master connection
		runCheck()

		plist, _ := GetActivePeers()

		//nominate first node as leader
		sort.Strings(plist)
		req := Request{}

		req.NodeType = EVENT_LEADER_NOMINATE
		req.Node = *localKnowPeers[plist[0]]
		req.FromNode = *getRaft().localNode

		log.Trace("time:", time.Now().Sub(lastNomiateTime).Seconds(), "node:", lastNomiateNode, " vs ", req.Node.RPCEndpoint)
		if lastNomiateNode == req.Node.RPCEndpoint && time.Now().Sub(lastNomiateTime).Seconds() < 30 {
			log.Debugf("skip to nominate the same node within 30s")
			return
		}

		log.Trace("I nominate: ", req.Node.RPCEndpoint)

		Broadcast(&req)

		lastNomiateNode = req.Node.RPCEndpoint
		lastNomiateTime = time.Now()
		return
	}
}

// Up joins a node, located at addr, to this store. The node must be ready to
// respond to Raft communications at that address.
func (s *RaftModule) Up(node *Node) error {
	f := getRaft().raft.AddPeer(node.RPCEndpoint)
	if f.Error() != nil {
		log.Trace(f.Error())
	}

	if _, ok := localKnowPeers[node.RPCEndpoint]; !ok {
		log.Tracef("ignore while local exists")
		return nil
	}

	localKnowPeers[node.RPCEndpoint].Active = true

	c := Command{}
	c.Op = NodeUp
	c.Key = node.RPCEndpoint
	c.Value = util.ToJson(node, false)
	s.ExecuteCommand(&c)
	return nil
}

func (s *RaftModule) Down(raftAddr, rpcAddr string) error {

	if _, ok := localKnowPeers[raftAddr]; !ok {
		return nil
	}

	localKnowPeers[raftAddr].Active = false

	c := Command{}
	c.Op = NodeDown
	c.Key = rpcAddr
	s.ExecuteCommand(&c)
	return nil
}

func (s *RaftModule) Leave(raftAddr, rpcAddr string) error {
	delete(localKnowPeers, raftAddr)

	f := getRaft().raft.RemovePeer(raftAddr)
	if f.Error() != nil {
		log.Error(f.Error())
	}

	c := Command{}
	c.Op = NodeLeave
	c.Key = rpcAddr
	s.ExecuteCommand(&c)
	return nil
}

func (s *RaftModule) ExecuteCommand(c *Command) error {

	log.Tracef("execute command: %v", c)

	if getRaft().raft.State() != raft.Leader {
		log.Tracef("I am not leader, skip")
		return fmt.Errorf("not leader")
	}

	//b, err := json.Marshal(c)
	//if err != nil {
	//	return err
	//}

	//f := s.raft.Apply(b, raftTimeout)
	//log.Tracef("apply command successful")
	//return f.Error()
	return nil
}

var l sync.Mutex

// SnapshotPersistID will make a snapshot and persist id stats to disk
func SnapshotClusterState() {
	l.Lock()
	defer l.Unlock()

	persistedPath := path.Join(global.Env().GetWorkingDir(), "_cluster")

	var buf bytes.Buffer
	err := gob.NewEncoder(&buf).Encode(getRaft().localNode)
	if err != nil {
		panic(err)
	}
	err = ioutil.WriteFile(persistedPath, buf.Bytes(), 0600)
	if err != nil {
		log.Error(persistedPath)
		panic(err)
	}
}

// RestorePersistID will take the snapshot and restore to id seeds
func RestoreClusterState() {
	l.Lock()
	defer l.Unlock()

	persistedPath := path.Join(global.Env().GetWorkingDir(), "_cluster")

	if !util.FileExists(persistedPath) {
		return
	}

	n, err := ioutil.ReadFile(persistedPath)
	if err != nil {
		panic(err)
	}

	buf := bytes.NewReader(n)
	err = gob.NewDecoder(buf).Decode(&getRaft().localNode)
	if err != nil {
		panic(err)
	}
}
