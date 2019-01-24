package cluster

import (
	"github.com/infinitbyte/framework/core/cluster"
	pb "github.com/infinitbyte/framework/core/cluster/pb"
	"github.com/infinitbyte/framework/core/config"
	"github.com/infinitbyte/framework/core/rpc"
	"github.com/infinitbyte/framework/modules/cluster/demo/server"
	"github.com/infinitbyte/framework/modules/cluster/discovery"
)

type ClusterModule struct {
}

func (module ClusterModule) Name() string {
	return "ClusterName"
}

func (module ClusterModule) Setup(cfg *config.Config) {

	rpc.Setup()
	cluster.New()
}

func (module ClusterModule) Start() error {

	server.Init()

	mys := &discovery.Discovery{}
	pb.RegisterDiscoveryServer(rpc.GetRPCServer(), mys)

	rpc.StartRPCServer()

	if err := cluster.Open(); err != nil {
		panic(err)
	}

	return nil
}

func (module ClusterModule) Stop() error {
	return nil
}
