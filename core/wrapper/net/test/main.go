package main

import (
	"flag"
	"github.com/j-keck/arping"
	net1 "infini.sh/framework/core/net"
	"net"
	"time"
)

var (
	device = flag.String("device", "eth0", "the network device")
	ip     = flag.String("ip", "192.168.3.199", "the network ip to listen")
	mask   = flag.String("mask", "255.255.255.0", "the network mask to listen")
)

//run with root user:
//./test -device=enp3s0 -ip=192.168.3.219
func main() {
	flag.Parse()
	net1.SetupAlias(*device, *ip, *mask)
	defer net1.DisableAlias(*device, *ip, *mask)

	ip := net.ParseIP(*ip)

	//https://www.practicalnetworking.net/series/arp/gratuitous-arp/
	//The Gratuitous ARP is sent as a broadcast, as a way for a node to announce or update its IP to MAC mapping to the entire network.
	err := arping.GratuitousArpOverIfaceByName(ip, *device)
	if err != nil {
		panic(err)
	}

	//if is active selected, do arping every 5s, or disable alias when not selected.

	time.Sleep(90 * time.Second)
}
