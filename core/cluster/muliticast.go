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

package cluster

import (
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/util"
	log "github.com/cihub/seelog"
	"net"
)

const (
	maxDataSize = 4096
)

//var lastBroadcast time.Time
//send a Broadcast message to network to discovery the cluster
func Broadcast(config config.NetworkConfig, req *Request) {
	//if time.Now().Sub(lastBroadcast).Seconds() < 5 {
	//	log.Warn("broadcast requests was throttled(5s)")
	//	return
	//}
	addr, err := net.ResolveUDPAddr("udp", config.GetBindingAddr())
	if err != nil {
		log.Error(err)
	}
	c, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		log.Error(err)
	}

	payload := util.ToJSONBytes(req)

	c.Write(payload)
	//lastBroadcast=time.Now()
}

func ServeMulticastDiscovery(config config.NetworkConfig, h func(*net.UDPAddr, int, []byte), signal chan bool) {

	addr, err := net.ResolveUDPAddr("udp", config.GetBindingAddr())
	if err != nil {
		log.Error(err)
	}

	l, err := net.ListenMulticastUDP("udp", nil, addr)
	if err != nil {
		log.Error(err)
	}

	l.SetReadBuffer(maxDataSize)

	signal <- true

	for {
		b := make([]byte, maxDataSize)
		n, src, err := l.ReadFromUDP(b)
		if err != nil {
			log.Error("read from UDP failed:", err)
		}
		h(src, n, b)
	}

}
