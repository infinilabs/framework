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

	// Setup Raft communication.
	//global.Env().SystemConfig.NetworkConfig.RaftBinding = util.AutoGetAddress(util.GetSafetyInternalAddress(global.Env().SystemConfig.NetworkConfig.RaftBinding))

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

	//log.Info("raft server listen at: ", getRaft().addr)

	//Hello world, I am up, where is the cluster?
	broadcast()

	//update local metadata
	//meta := GetClusterReadonlyMetadata()
	//for k, v := range meta.KnownNodesRPCEndpoint {
	//	localKnowPeers[k] = v
	//}

	//How are you, I'll reach to you
	health()

	registerNode(getRaft().localNode.RaftEndpoint, getRaft().localNode)

	return err
}

func broadcast() {

	req := Request{}

	node :=
		Node{
			APIEndpoint:  util.GetSafetyInternalAddress(global.Env().SystemConfig.NetworkConfig.APIBinding),
			RPCEndpoint:  util.GetSafetyInternalAddress(global.Env().SystemConfig.NetworkConfig.RPCBinding),
			RaftEndpoint: util.GetSafetyInternalAddress(global.Env().SystemConfig.NetworkConfig.RPCBinding),
			ClusterName:  global.Env().SystemConfig.ClusterConfig.Name,
			StartTime:    env.GetStartTime().Unix(),
			Name:         global.Env().SystemConfig.NodeConfig.Name}

	req.Node = node
	req.NodeType = onlineNodeType
	getRaft().localNode = &node
	Broadcast(&req)

}

func health() {

	//if listen as leader, start heartbeat service
	go func() {

	start_loop:
		s := getRaft()

		localRpc := util.GetSafetyInternalAddress(global.Env().SystemConfig.NetworkConfig.RPCBinding)

		for {
			if s.raft == nil || s.raft.Leader() != "" && s.raft.Leader() != s.addr {
				log.Trace("I am not qualified to run the health check, skip")
				goto quit_loop
			}

			//try nomination
			if s.raft.Leader() == "" {
				//log.Trace("let's find the leader")
				if GetLocalActivePeersCount() > 0 {
					//try nominate
					nominate()
				} else {
					//where is cluster
					broadcast()
				}

				goto quit_loop
				continue
			}

			log.Tracef("start heartbeat check, %v hosts", len(localKnowPeers))

			for i, v := range localKnowPeers {
				log.Trace("heartbeat:", i, ",", v, ",", v.RPCEndpoint, " vs ", localRpc)

				previousState := v.Active

				////ignore local node
				if v.RPCEndpoint == localRpc {

					//if I am leader and I am not in the list, add it
					if v, ok := localKnowPeers[v.RaftEndpoint]; !ok && !v.Active {
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
					getRaft().Leave(v.RaftEndpoint, v.RPCEndpoint)
					//panic(err)
				}

				defer conn.Close()
				c := pb.NewDiscoveryClient(conn)

				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				r, err := c.Ping(ctx, &pb.HealthCheckRequest{
					NodeName: global.Env().SystemConfig.NodeConfig.Name,
					NodeIp:   s.tcpAddr.IP.String(),
					NodePort: uint32(s.tcpAddr.Port),
				})

				if r == nil || (err != nil && previousState == true) {
					log.Error(v.RPCEndpoint, ",", err)
					v.Active = false
					getRaft().Down(v.RaftEndpoint, v.RPCEndpoint)
					continue
				}

				if r.Success && !previousState {
					v.Active = true
					getRaft().Up(v)
				}
				log.Trace("heartbeat success,", r.Success)

			}

			time.Sleep(time.Second * 10)
		}

	quit_loop:
		log.Trace("pause heartbeat service")
		time.Sleep(time.Second * 10)
		goto start_loop
	}()
}

func (s *RaftModule) restart(seeds []string) (err error) {

	if time.Now().Sub(electedTime).Seconds() < 30 {
		log.Debug("skip election, already in progress")
		return
	}

	log.Trace("restart with seeds,", seeds)

	//if s.transport != nil && !s.transport.IsShutdown() {
	//err = s.transport.Close()
	//if err != nil {
	//	log.Error(err)
	//	return err
	//}

	if s.raft != nil && s.raft.State() != raft.Shutdown {
		err = s.raft.Shutdown().Error()
		if err != nil {
			log.Error(err)
			return err
		}

	}
	//}

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
	s.raft = ra

	log.Trace("raft initialized")

	//started = true
	electedTime = time.Now()
	return nil
}

var localKnowPeers = make(map[string]*Node)
var ticketsBox = hashset.New()

const onlineNodeType = "ONLINE"
const nominateNodeType = "NOMINATE"
const leaderNodeType = "LEADER"

var electedTime time.Time
//var started bool

func (s *RaftModule) multicastCallback(src *net.UDPAddr, n int, b []byte) {

	v := Request{}

	util.FromJSONBytes(b[:n], &v)

	log.Tracef("received multicast message: %v", v)

	if s.raft == nil {
		log.Error("raft is not initialized")
		return
	}

	//leader: add to local peers
	raftAddr := v.Node.RaftEndpoint

	v.Node.Active = true

	//if(raftAddr==getRaft().addr){
	//	log.Trace("ignore local message")
	//	return
	//}

	//send message back to new nodes
	if v.NodeType == onlineNodeType {
		if getRaft().raft.Leader() == getRaft().addr {
			localKnowPeers[raftAddr] = &v.Node
			req := Request{}

			req.Node = v.Node
			req.FromNode = *getRaft().localNode
			req.NodeType = leaderNodeType

			Broadcast(&req)
			getRaft().raft.AddPeer(v.Node.RaftEndpoint)
		}
	}

	if v.NodeType == nominateNodeType {

		l.Lock()
		defer l.Unlock()

		log.Tracef("check nominate message, %v vs %v ", raftAddr, s.addr)

		if v.FromNode.RaftEndpoint == s.addr && getRaft().raft.Leader() != "" {
			registerNode(v.FromNode.RaftEndpoint, &v.FromNode)
		}

		//if the nominate is for self, tickets +1
		if raftAddr == s.addr && getRaft().raft.Leader() == "" {
			ticketsBox.Add(v.FromNode.RaftEndpoint)
			log.Debugf("i am nominated as leader, got ticket: %v ,from:%s", ticketsBox.Size(), src.String())
			metadata := GetClusterReadonlyMetadata()

			//if get quorum tickets, start to runLeader
			if ticketsBox.Size() >= ((len(metadata.KnownNodesRPCEndpoint)+1)/2) &&
				ticketsBox.Size() >= ((GetLocalActivePeersCount()+1)/2) &&
				GetLocalActivePeersCount() > 0 &&
				GetLocalActivePeersCount() >= global.Env().SystemConfig.ClusterConfig.MinimumNodes {

				log.Debug("got enough tickets, restart as leader now")
				//s.raft.Shutdown()
				//s.cfg.StartAsLeader = true
				seeds, _ := GetActivePeers()
				err := s.restart(seeds)
				if err != nil {
					panic(err)
					log.Error(err)
				}
				ticketsBox.Clear()
			}
			//get ticket, add to known list
			getRaft().raft.AddPeer(raftAddr)
		}

	}

	//process message from leader
	if v.NodeType == leaderNodeType {
		if v.Node.RaftEndpoint == getRaft().addr {
			registerNode(raftAddr, &v.FromNode)
			seeds, _ := GetActivePeers()
			err := getRaft().restart(seeds)
			if err != nil {
				panic(err)
			}
			return
		}
	}

	registerNode(raftAddr, &v.Node)

}

//register local node status
func registerNode(raftAddr string, node *Node) {
	if add, ok := localKnowPeers[raftAddr]; !ok {

		node.Active = true
		if node.ClusterName != global.Env().SystemConfig.ClusterConfig.Name {
			log.Error("cluster name mismatch, ignore node,", global.Env().SystemConfig.ClusterConfig.Name, " vs ", node.ClusterName)
			return
		}

		log.Tracef("add %s to know peers", raftAddr)

		localKnowPeers[raftAddr] = node

		if getRaft().raft.State() == raft.Leader {
			//auto join new nodes
			getRaft().Up(node)
			log.Error("node join")
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

		plist, _ := GetActivePeers()

		//nominate first node as leader
		sort.Strings(plist)
		req := Request{}

		req.NodeType = nominateNodeType
		req.Node = *localKnowPeers[plist[0]]
		req.FromNode = *getRaft().localNode

		log.Trace("time:", time.Now().Sub(lastNomiateTime).Seconds(), "node:", lastNomiateNode, " vs ", req.Node.RaftEndpoint)
		if lastNomiateNode == req.Node.RaftEndpoint && time.Now().Sub(lastNomiateTime).Seconds() < 30 {
			log.Debugf("skip to nominate the same node within 30s")
			return
		}

		log.Trace("I nominate: ", req.Node.RaftEndpoint)

		//add leader to my local peer
		//getRaft().raft.AddPeer(req.Node.RaftEndpoint)

		Broadcast(&req)

		lastNomiateNode = req.Node.RaftEndpoint
		lastNomiateTime = time.Now()

		seeds, _ := GetActivePeers()
		getRaft().restart(seeds)
		return
	}
}

// Up joins a node, located at addr, to this store. The node must be ready to
// respond to Raft communications at that address.
func (s *RaftModule) Up(node *Node) error {

	if v, ok := localKnowPeers[node.RaftEndpoint]; !ok && v.Active {
		log.Tracef("ignore while local exists")
		return nil
	}

	localKnowPeers[node.RaftEndpoint].Active = true

	f := s.raft.AddPeer(node.RaftEndpoint)
	if f.Error() != nil {
		log.Trace(f.Error())
	}

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

	f := s.raft.RemovePeer(raftAddr)
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

	log.Infof("execute command: %v", c)

	if s.raft.State() != raft.Leader {
		log.Errorf("I am not leader, skip")
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
	if !util.FileExists(persistedPath) {
		//return
	}

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
