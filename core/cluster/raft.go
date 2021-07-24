package cluster

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/emirpasic/gods/sets/hashset"
	pb "infini.sh/framework/core/cluster/pb"
	"infini.sh/framework/core/cluster/raft"
	raftboltdb "infini.sh/framework/core/cluster/raft-boltdb"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/rpc"
	"infini.sh/framework/core/stats"
	"infini.sh/framework/core/util"
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

var clusterConfig *config.ClusterConfig

func New(config *config.ClusterConfig) *RaftModule {

	clusterConfig = config
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

//other peers, self not include
func GetPeers() []string {
	return getRaft().raft.Peers()
}

//get all nodes, include self
func GetNodes() []string {
	return getRaft().raft.Nodes()
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
func RefreshAllKnowPeers() {

	for _, targetEndpoint := range getRaft().raft.Peers() {
		_, ok := localKnowPeers[targetEndpoint]
		if !ok {
			v := Node{
				Name:        "unknow",
				RPCEndpoint: targetEndpoint,
				Active:      false,
			}
			localKnowPeers[targetEndpoint] = &v
		}
	}
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

var discoveryStartTime = time.Now()

func Open() (err error) {

	getRaft().addr = rpc.GetRPCAddress()

	getRaft().tcpAddr, err = net.ResolveTCPAddr("tcp", getRaft().addr)
	if err != nil {
		panic(err)
	}

	//wait to get enough peers
	signal := make(chan bool)
	go ServeMulticastDiscovery(clusterConfig.BoradcastConfig, getRaft().multicastCallback, signal)
	<-signal

	var seeds []string
	if len(global.Env().SystemConfig.ClusterConfig.GetSeeds()) > 0 {
		log.Tracef("use config seeds")
		seeds = global.Env().SystemConfig.ClusterConfig.GetSeeds()
	}

	if GetLocalActivePeersCount() > 1 && GetLocalActivePeersCount() >= global.Env().SystemConfig.ClusterConfig.MinimumNodes {
		seeds, _ = GetActivePeers()
		log.Tracef("prefer use local active peers as seeds, %v", len(seeds))
	}

	err = getRaft().restart(seeds, RESTART_WITH_INITIAL_STATE)
	if err != nil {
		log.Error(err)
		panic(err)
	}

	////Hello world, I am up, where is the cluster out there?
	discoveryCluster()

	//How are you, I'll reach to you
	runDiscoveryTasks()

	registerNode(getRaft().localNode)

	return err
}

var lastDiscoveried time.Time

func discoveryCluster() {

	if time.Since(lastDiscoveried) < 5*time.Second {
		log.Debug("discovery request already sent within 5 sends, skip")
		return
	}

	//if discovered{
	//	log.Debug("already sent discovery request, skip")
	//	return
	//}
	req := Request{}

	node :=
		Node{
			//APIEndpoint: util.GetSafetyInternalAddress(global.Env().SystemConfig.NetworkConfig.APIBinding),
			RPCEndpoint: util.GetSafetyInternalAddress(rpc.GetRPCAddress()),
			ClusterName: global.Env().SystemConfig.ClusterConfig.Name,
			StartTime:   env.GetStartTime().Unix(),
			Name:        global.Env().SystemConfig.NodeConfig.Name}

	req.Node = node
	req.NodeType = EVENT_LEADER_DISCOVERY
	getRaft().localNode = &node
	Broadcast(clusterConfig.BoradcastConfig, &req)

	lastDiscoveried = time.Now()
}

var selfPromoted bool

func runDiscoveryTasks() {

	//if listen as leader, start health check service
	go func() {
	start_loop:

		for {
			if getRaft().raft == nil || getRaft().raft.Leader() != "" && getRaft().raft.Leader() != getRaft().addr {
				log.Trace("I am not qualified to run the health check, skip")
				time.Sleep(30 * time.Second)
				goto quit_loop
			}

			if getRaft().raft.Leader() == "" {
				log.Debug("leader is not selected, try nominate one")
				//try discovery first, and then do nominate
				if GetLocalActivePeersCount() > 0 {
					if time.Since(discoveryStartTime).Milliseconds() > global.Env().SystemConfig.ClusterConfig.DiscoveryTimeoutInMilliseconds {
						//try nominate
						nominate()
					} else {

						timeToWait := global.Env().SystemConfig.ClusterConfig.DiscoveryTimeoutInMilliseconds - time.Since(discoveryStartTime).Milliseconds()

						if timeToWait > 0 {
							time.Sleep(time.Millisecond * time.Duration(timeToWait))
							log.Info("still waiting for discovery result, or start nominate process after ", time.Millisecond*time.Duration(timeToWait))
						}
					}

				} else {
					//where is cluster
					discoveryCluster()
				}

				goto quit_loop
				continue
			} else {
				//leader will do health check on all the connections
				runHealthCheck() //TODO open it after debug
			}

			log.Trace("pause health check service for 10s")
			time.Sleep(time.Second * 10)
		}

	quit_loop:

		//self promote should be used for only once
		if selfElectable() && selfPromoted == false {

			timeToWait := global.Env().SystemConfig.ClusterConfig.DiscoveryTimeoutInMilliseconds - time.Since(discoveryStartTime).Milliseconds()

			if timeToWait > 0 {
				log.Debugf("wait %s to try self promotion,  ", time.Millisecond*time.Duration(timeToWait))
				time.Sleep(time.Millisecond * time.Duration(timeToWait))
			}

			if getRaft().raft.Leader() == "" && GetLocalActivePeersCount() <= 0 && !skipSelfPromotion {
				log.Debugf("start to do self promotion")
				seeds, _ := GetActivePeers()
				err := getRaft().restart(seeds, RESTART_WITH_SELFT_PROMOTE)
				if err != nil {
					panic(err)
				}
				selfPromoted = true
			}
		}

		log.Trace("pause health check service for ", time.Millisecond*time.Duration(global.Env().SystemConfig.ClusterConfig.HealthCheckInMilliseconds))
		time.Sleep(time.Millisecond * time.Duration(global.Env().SystemConfig.ClusterConfig.HealthCheckInMilliseconds))

		goto start_loop
	}()
}

func runHealthCheck() {
	RefreshAllKnowPeers()

	peers := localKnowPeers
	if len(peers) == 0 {
		return
	}

	log.Tracef("start health check to %v hosts", len(peers))
	localRpc := util.GetSafetyInternalAddress(rpc.GetRPCAddress())

	for targetEndpoint, v := range peers {
		log.Debug("health check: ", v, ", remote: ", v, " local: ", localRpc)

		////ignore local node
		if targetEndpoint == localRpc {

			log.Trace("ignore self check")
			continue
		}

		client, err := rpc.ObtainConnection(targetEndpoint)
		if err != nil || client == nil {
			log.Error("fail to obtain connection")
			continue
			v.Active = false
			getRaft().Leave(targetEndpoint, targetEndpoint)
			stats.Absolute("node_countdown", targetEndpoint, 0)
		}
		defer client.Close()
		c := pb.NewDiscoveryClient(client.ClientConn)

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		log.Tracef("send ping request: %v", targetEndpoint)

		r, err := c.Ping(ctx, &pb.HealthCheckRequest{
			NodeName: v.Name,
			NodeIp:   v.GetRPCAddr().IP.String(),
			NodePort: uint32(v.GetRPCAddr().Port),
		})

		if err != nil {
			log.Trace(err)
		}

		previousState := localKnowPeers[targetEndpoint].Active
		if r == nil || err != nil {

			stats.Increment("node_countdown", targetEndpoint)
			count := stats.Stat("node_countdown", targetEndpoint)
			log.Errorf("%v unable reach for %v times, %v", targetEndpoint, count, err)
			v.Active = false

			if count >= 3 {
				log.Debugf("node %v unable to reach for %v times, mark it was down", targetEndpoint, count)
				getRaft().Down(targetEndpoint, targetEndpoint)
				stats.Absolute("node_countdown", targetEndpoint, 0)
			}
			continue
		}

		if r.Success && !previousState {
			v.Active = true
			getRaft().Up(v)
			stats.Absolute("node_countdown", targetEndpoint, 0)
		}
		log.Trace("all health check tasks are done")
	}
}

var restartLock sync.Mutex

func (s *RaftModule) shutdownRaft() error {
	if getRaft().raft != nil && getRaft().raft.State() != raft.Shutdown {
		return getRaft().raft.Shutdown().Error()
	}
	return nil
}

func (s *RaftModule) startRaft(seeds []string) error {
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

	dir := path.Join(global.Env().GetDataDir(), "raft")

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
	//electedTime = time.Now()
	return nil
}

func (s *RaftModule) restart(seeds []string, t RESTART_TYPE) (err error) {

	log.Trace("restart leader selection or discovery process, type: ", t, " seeds: ", seeds)

	restartLock.Lock()
	defer restartLock.Unlock()

	if t == RESTART_WITH_INITIAL_STATE {

		err = s.startRaft(seeds)
		if err != nil {
			log.Error(err)
			return err
		}

	} else if t == RESTART_WITH_KNOWN_LEADER {
		err := s.shutdownRaft()
		if err != nil {
			log.Error(err)
			return err
		}

		err = s.startRaft(seeds)
		if err != nil {
			log.Error(err)
			return err
		}

	} else if t == RESTART_WITH_LEADER_SELECTION {

		err := s.shutdownRaft()
		if err != nil {
			log.Error(err)
			return err
		}

		err = s.startRaft(seeds)
		if err != nil {
			log.Error(err)
			return err
		}

	} else if t == RESTART_WITH_SELFT_PROMOTE {

		log.Debug("start to do self promote")

		//after 30s(configurable) timeout, if there are no other peers show up, do promote self
		if time.Since(discoveryStartTime).Milliseconds() >= global.Env().SystemConfig.ClusterConfig.DiscoveryTimeoutInMilliseconds && selfElectable() && len(seeds) == 0 {

			err := s.shutdownRaft()
			if err != nil {
				log.Error(err)
				return err
			}

			if skipSelfPromotion {
				return nil
			} else {
				log.Warnf("can't discovery a valid cluster, will be act as leader")
				s.cfg.StartAsLeader = true
			}

			seeds, _ = GetActivePeers()
			err = s.startRaft(seeds)
			if err != nil {
				log.Error(err)
				return err
			}
		}
	} else {
		panic("unknow restart type")
	}

	return nil
}

//check if self promote was enabled
func selfElectable() bool {

	return global.Env().SystemConfig.ClusterConfig.MinimumNodes <= 1
}

var localKnowPeers = make(map[string]*Node)
var ticketsBox = hashset.New()

const EVENT_LEADER_DISCOVERY = "LEADER_DISCOVERY"
const EVENT_LEADER_ECHO = "LEADER_ECHO"
const EVENT_LEADER_NOMINATE = "LEADER_NOMINATE"
const EVENT_LEADER_READY = "LEADER_READY"

var electedTime time.Time
var skipSelfPromotion bool

type RESTART_TYPE string

const RESTART_WITH_KNOWN_LEADER RESTART_TYPE = "RESTART_WITH_KNOWN_LEADER"
const RESTART_WITH_LEADER_SELECTION RESTART_TYPE = "RESTART_WITH_LEADER_SELECTION"
const RESTART_WITH_INITIAL_STATE RESTART_TYPE = "RESTART_WITH_INITIAL_STATE"
const RESTART_WITH_SELFT_PROMOTE RESTART_TYPE = "RESTART_WITH_SELFT_PROMOTE"

func (s *RaftModule) multicastCallback(src *net.UDPAddr, n int, b []byte) {

	v := Request{}

	util.MustFromJSONBytes(b[:n], &v)

	log.Tracef("received multicast message: %v", util.ToJson(v, false))

	if getRaft().raft == nil {
		log.Error("raft is not initialized")
		return
	}

	//ignore my broadcast message
	if v.FromNode.RPCEndpoint == s.addr {
		log.Trace("ignore my broadcast message")
		return
	}

	//node is active
	v.Node.Active = true
	registerNode(&v.Node)

	switch v.NodeType {

	case EVENT_LEADER_DISCOVERY:

		if getRaft().raft.Leader() == getRaft().addr && getRaft().raft.Leader() != "" {
			log.Debugf("%v is up, let me send a greeting message", v.Node.RPCEndpoint)
			registerNode(&v.Node)
			getRaft().raft.AddPeer(v.Node.RPCEndpoint)

			req := Request{}
			req.Node = v.Node
			req.FromNode = *getRaft().localNode
			req.NodeType = EVENT_LEADER_ECHO

			Broadcast(clusterConfig.BoradcastConfig, &req)

		}

		break
	case EVENT_LEADER_ECHO:
		if v.Node.RPCEndpoint == getRaft().addr && getRaft().raft.Leader() == "" {
			log.Debug("received a message from leader, bingo")
			registerNode(&v.FromNode)
			skipSelfPromotion = true
			seeds, _ := GetActivePeers()
			err := getRaft().restart(seeds, RESTART_WITH_KNOWN_LEADER)
			if err != nil {
				panic(err)
			}
			return
		}

		break
	case EVENT_LEADER_READY:
		log.Tracef("leader %v is ready", v.Node.RPCEndpoint)
		if getRaft().raft.Leader() == "" {
			seeds := []string{v.Node.RPCEndpoint}
			err := getRaft().restart(seeds, RESTART_WITH_KNOWN_LEADER)
			if err != nil {
				panic(err)
			}
		}

		break
	case EVENT_LEADER_NOMINATE:
		l.Lock()
		defer l.Unlock()

		log.Tracef("seems %s is nominating to be %s leader", v.FromNode.RPCEndpoint, v.Node.RPCEndpoint)

		registerNode(&v.FromNode)

		//if i am the leader, i will tell these nodes, don't bother, come and join me
		if getRaft().raft.Leader() == s.addr && getRaft().raft.Leader() != "" {
			log.Debugf("no bother looking，i am the leader, let me add you to my group, let's have fun together")

			getRaft().raft.AddPeer(v.Node.RPCEndpoint)
			getRaft().raft.AddPeer(v.FromNode.RPCEndpoint)

			req := Request{}
			req.Node = v.Node
			req.FromNode = *getRaft().localNode
			req.NodeType = EVENT_LEADER_ECHO

			Broadcast(clusterConfig.BoradcastConfig, &req)
			return
		}

		//if the nominate is for self, tickets +1, and can't nominated by self
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

				log.Debug("tickets enough，I am the leader now，let's do it!")

				err := s.restart(seeds, RESTART_WITH_LEADER_SELECTION)
				if err != nil {
					panic(err)
				}

				ticketsBox.Clear()

				req := Request{}
				req.Node = *getRaft().localNode
				req.NodeType = EVENT_LEADER_READY
				Broadcast(clusterConfig.BoradcastConfig, &req)
			}
		}

		break
	}
}

//register local node status
func registerNode(node *Node) {
	//skip adding self
	if node.RPCEndpoint == getRaft().addr {
		return
	}

	if add, ok := localKnowPeers[node.RPCEndpoint]; !ok {

		if node.ClusterName != global.Env().SystemConfig.ClusterConfig.Name {
			log.Error("cluster name mismatch, ignore node,", global.Env().SystemConfig.ClusterConfig.Name, " vs ", node.ClusterName)
			return
		}

		log.Tracef("add %s to know peers", node.RPCEndpoint)

		localKnowPeers[node.RPCEndpoint] = node

		if getRaft().raft.State() == raft.Leader {
			//auto join new nodes
			node.Active = true
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
		runHealthCheck()

		plist, _ := GetActivePeers()

		//nominate first node as leader
		sort.Strings(plist)
		req := Request{}
		if GetLocalActivePeersCount() <= 0 {
			return
		}

		req.NodeType = EVENT_LEADER_NOMINATE
		req.Node = *localKnowPeers[plist[0]]
		req.FromNode = *getRaft().localNode

		if req.Node.RPCEndpoint == getRaft().addr {
			log.Debugf("skip to nominate self")
			return
		}

		log.Trace("time:", time.Now().Sub(lastNomiateTime).Seconds(), ", node:", lastNomiateNode, " vs ", req.Node.RPCEndpoint)
		if lastNomiateNode == req.Node.RPCEndpoint && time.Now().Sub(lastNomiateTime).Seconds() < 30 {
			log.Debugf("skip to nominate the same node within 30s")
			return
		}

		log.Debug("hey, anyone wanna be leader, I am going to nominate: ", req.Node.RPCEndpoint)

		Broadcast(clusterConfig.BoradcastConfig, &req)

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

	return nil
}

func (s *RaftModule) Down(raftAddr, rpcAddr string) error {

	if _, ok := localKnowPeers[raftAddr]; !ok {
		return nil
	}

	localKnowPeers[raftAddr].Active = false

	return nil
}

func (s *RaftModule) Leave(raftAddr, rpcAddr string) error {
	delete(localKnowPeers, raftAddr)

	f := getRaft().raft.RemovePeer(raftAddr)
	if f.Error() != nil {
		log.Error(f.Error())
	}

	return nil
}

var l sync.Mutex

// SnapshotPersistID will make a snapshot and persist id stats to disk
func SnapshotClusterState() {
	l.Lock()
	defer l.Unlock()

	persistedPath := path.Join(global.Env().GetDataDir(), "_cluster")

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

	persistedPath := path.Join(global.Env().GetDataDir(), "_cluster")

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
