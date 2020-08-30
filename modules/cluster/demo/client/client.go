/*
Copyright Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"log"
	"os"
	"time"

	"flag"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/rpc"
	"infini.sh/framework/core/util"
	pb "infini.sh/framework/modules/cluster/demo/helloworld"
)

const (
	address     = "localhost:50051"
	defaultName = "world"
)

var addr = flag.String("bind", "localhost:10000", "the rpc address to bind to")

func main() {
	util.RestorePersistID("/tmp")

	global.RegisterEnv(env.EmptyEnv().SetConfigFile("config.yml"))

	rpc.Setup()

	conn, err := rpc.ObtainConnection(*addr)
	if err != nil {
		log.Fatalf("could not greet: %v", err)
	}

	defer conn.Close()

	c := pb.NewGreeterClient(conn.ClientConn)

	// Contact the server and print out its response.
	name := defaultName
	if len(os.Args) > 1 {
		name = os.Args[1]
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	r, err := c.SayHello(ctx, &pb.HelloRequest{Name: name})
	if err != nil {
		log.Fatalf("could not greet: %v", err)
	}
	log.Printf("Greeting: %s", r.Message)
}
