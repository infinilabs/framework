package cluster

import (
	"context"
	"encoding/json"
	"fmt"
	log "github.com/cihub/seelog"
	pb "github.com/infinitbyte/framework/core/cluster/pb"
	"github.com/infinitbyte/framework/core/cluster/raft"
	"github.com/infinitbyte/framework/core/cluster/raft-boltdb"
	"github.com/infinitbyte/framework/core/errors"
	"github.com/infinitbyte/framework/core/global"
	"github.com/infinitbyte/framework/core/rpc"
	"github.com/infinitbyte/framework/core/util"
	"net"
	"os"
	"path"
	"path/filepath"
	"sort"
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
	fsm       raft.FSM
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

func GetActivePeersCount() int {
	count := 0
	for _, v := range knowPeers {
		if v.Active {
			count++
		}
	}
	return count
}

func GetActivePeers() map[string]*Node {
	return knowPeers
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

func (s *RaftModule) Open() (err error) {

	// Setup Raft communication.
	s.addr = util.AutoGetAddress(global.Env().SystemConfig.NetworkConfig.RaftBinding)

	s.tcpAddr, err = net.ResolveTCPAddr("tcp", s.addr)
	if err != nil {
		panic(err)
	}

	s.restart()

	signal := make(chan bool)

	go ServeMulticastDiscovery(s.multicastCallback, signal)

	<-signal

	s.health()

	return err
}

func (s *RaftModule) Broadcast() {

	//wait 10s to let RPC service startup
	time.Sleep(10 * time.Second)

	req := Request{}
	req.Node =
		Node{
			APIEndpoint:  global.Env().SystemConfig.NetworkConfig.APIBinding,
			RPCEndpoint:  global.Env().SystemConfig.NetworkConfig.RPCBinding,
			RaftEndpoint: global.Env().SystemConfig.NetworkConfig.RaftBinding,
			ClusterName:  global.Env().SystemConfig.ClusterConfig.Name,
			Name:         global.Env().SystemConfig.NodeConfig.Name}

	Broadcast(&req)

}

func (s *RaftModule) health() {

	//if listen as leader, start heartbeat service
	go func() {

	start_loop:

		for {
			log.Trace("start heartbeat check")
			if s.raft.Leader() != "" && s.raft.Leader() != s.addr {
				goto quit_loop
			}

			for i, v := range knowPeers {
				log.Trace("heartbeat:", i, ",", v)

				//ignore local node
				if v.RPCEndpoint == s.addr {
					continue
				}
				conn, err := rpc.ObtainConnection(v.RPCEndpoint)
				if err != nil {
					panic(err)
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

				if err != nil {
					log.Error(err)
					v.Active = false
					continue
				}

				if r.Success {
					v.Active = true
				}

			}

			time.Sleep(time.Second * 10)
		}

	quit_loop:
		log.Trace("pause heartbeat service")
		time.Sleep(time.Second * 10)
		goto start_loop
	}()
}

func (s *RaftModule) restart() (err error) {

	if s.transport != nil && !s.transport.IsShutdown() {
		err = s.transport.Close()
		if err != nil {
			log.Error(err)
		}
	}

	global.Env().SystemConfig.NetworkConfig.RaftBinding = s.addr
	addr, err := net.ResolveTCPAddr("tcp", s.addr)
	if err != nil {
		log.Error(err)
		return err
	}

	s.transport, err = raft.NewTCPTransport(s.addr, addr, 3, 10*time.Second, os.Stderr)
	if err != nil {
		log.Error(err)
		return err
	}

	log.Info("raft server listen at: ", s.addr)

	if !global.Env().IsDebug {
		//disable raft logging
		s.cfg.LogOutput = new(NullWriter)
	}

	peerStore := &raft.StaticPeers{StaticPeers: global.Env().SystemConfig.ClusterConfig.Seeds}

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
	ra, err := raft.NewRaft(s.cfg, s.fsm, logStore, logStore, snapshots, peerStore, s.transport)
	if err != nil {
		return fmt.Errorf("new raft: %s", err)
	}
	s.raft = ra

	return nil
}

var knowPeers = make(map[string]*Node)
var ticketsNum int

const nominateNodeType = "NOMINATE"

func (s *RaftModule) multicastCallback(src *net.UDPAddr, n int, b []byte) {

	v := Request{}

	util.FromJSONBytes(b[:n], &v)

	log.Tracef("received multicast message: %v", v)

	//leader: add to local peers
	addr := v.Node.RaftEndpoint

	if v.NodeType == nominateNodeType {
		//if the nominate is for self, tickets +1
		if addr == s.addr {
			ticketsNum++
			log.Debugf("i am nominated as leader, got ticket: %v ,from:%s", ticketsNum, src.String())

			//if get quorum tickets, start to runLeader
			if ticketsNum >= ((len(knowPeers)+1)/2) && GetActivePeersCount() >= global.Env().SystemConfig.ClusterConfig.MinimumNodes {
				log.Debug("got enough tickets, restart as leader now")
				s.raft.Shutdown()
				s.cfg.StartAsLeader = true
				s.restart()
				ticketsNum = 0
			}
		}
	}

	if add, ok := knowPeers[addr]; !ok {

		v.Node.Active = true
		if v.Node.ClusterName != global.Env().SystemConfig.ClusterConfig.Name {
			log.Error("cluster name mismatch, ignore node,", v, ",")
			return
		}

		log.Tracef("add %s to know peers", addr)

		knowPeers[addr] = &v.Node

		if s.raft.Leader() == "" && GetActivePeersCount() >= global.Env().SystemConfig.ClusterConfig.MinimumNodes {
			log.Debugf("met enough raft nodes(%v), start nomination", global.Env().SystemConfig.ClusterConfig.MinimumNodes)
			for k := range knowPeers {
				global.Env().SystemConfig.ClusterConfig.Seeds = append(global.Env().SystemConfig.ClusterConfig.Seeds, k)
			}
			//nominate first node as leader
			sort.Strings(global.Env().SystemConfig.ClusterConfig.Seeds)
			req := Request{}

			req.NodeType = nominateNodeType
			req.Node = *knowPeers[global.Env().SystemConfig.ClusterConfig.Seeds[0]]

			Broadcast(&req)
			return
		}

		if s.raft.State() == raft.Leader {
			//auto join new nodes
			s.Join(addr)
		}
	} else {
		add.Active = true
	}
}

// Join joins a node, located at addr, to this store. The node must be ready to
// respond to Raft communications at that address.
func (s *RaftModule) Join(addr string) error {
	f := s.raft.AddPeer(addr)
	if f.Error() != nil {
		log.Error(f.Error())
		return f.Error()
	}
	return nil
}

func (s *RaftModule) Remove(addr string) error {
	f := s.raft.RemovePeer(addr)
	if f.Error() != nil {
		log.Error(f.Error())
		return f.Error()
	}
	return nil
}

func (s *RaftModule) ExecuteCommand(c *Command) error {

	log.Infof("execute command: %v", c)

	if s.raft.State() != raft.Leader {
		log.Infof("not leader, skip")
		return fmt.Errorf("not leader")
	}

	b, err := json.Marshal(c)
	if err != nil {
		return err
	}

	f := s.raft.Apply(b, raftTimeout)
	log.Infof("apply command successful")
	return f.Error()
}
