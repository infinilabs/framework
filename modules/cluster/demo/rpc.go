/*
Copyright 2016 Medcl (m AT medcl.net)

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
package demo

//
//import (
//	"github.com/infinitbyte/framework/core/config"
//	"github.com/infinitbyte/framework/core/rpc"
//)
//
//func (module RPCModule) Name() string {
//	return "RPC"
//}
//
//func (module RPCModule) Setup(cfg *config.Config) {
//	rpc.Setup()
//
//}
//
//func (module RPCModule) Start() error {
//
//
//
//	//time.Sleep(5 * time.Second)
//	//
//	//conn, err := rpc.ObtainLocalConnection()
//	//if err != nil {
//	//	panic(err)
//	//}
//	//fmt.Println("get connection")
//	//
//	//defer conn.Close()
//	//c := demo.NewGreeterClient(conn)
//	//fmt.Println("get client")
//	//
//	//ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	//defer cancel()
//	//fmt.Println("start exe")
//	//
//	//r, err := c.SayHello(ctx, &demo.HelloRequest{Name: "medcl"})
//	//
//	//fmt.Println("end exe")
//	//
//	//if err != nil {
//	//	panic(err)
//	//}
//	//fmt.Println("Greeting: %s", r.Message)
//
//	return nil
//}
//
//func (module RPCModule) Stop() error {
//	return nil
//}
//
//type RPCModule struct {
//}
