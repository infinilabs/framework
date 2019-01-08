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

package multicast

//
//import (
//	"encoding/hex"
//	"github.com/infinitbyte/framework/core/cluster"
//	"github.com/infinitbyte/framework/core/env"
//	"github.com/infinitbyte/framework/core/util"
//	"log"
//	"net"
//	"time"
//)
//
//
//
//func main() {
//
//	go ServeMulticastDiscovery(multicastCallback)
//
//	req := cluster.Request{}
//	req.Node = cluster.Node{IP: "localhost", Port: 1235}
//
//	for i := 1; i < 1000; i++ {
//		Broadcast(&req)
//	}
//
//	time.Sleep(1 * time.Minute)
//}
