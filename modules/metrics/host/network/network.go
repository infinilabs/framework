/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package network

import (
	"strings"
	"syscall"

	log "github.com/cihub/seelog"
	"github.com/shirou/gopsutil/v3/net"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/util"
)

type Metric struct {
	interfaces   map[string]struct{}
	prevCounters networkCounter

	Enabled    bool     `config:"enabled"`
	Summary    bool     `config:"summary"`
	Sockets bool     `config:"sockets"`
	Throughput bool     `config:"throughput"`
	Detail     bool     `config:"details"`
	Interfaces []string `config:"interfaces"`
}

type networkCounter struct {
	prevNetworkInBytes    uint64
	prevNetworkInPackets  uint64
	prevNetworkOutBytes   uint64
	prevNetworkOutPackets uint64
}

func New(cfg *config.Config) (*Metric, error) {

	me := &Metric{
		Enabled:      true,
		prevCounters: networkCounter{},
	}

	err := cfg.Unpack(&me)
	if err != nil {
		panic(err)
	}

	me.interfaces = make(map[string]struct{}, len(me.Interfaces))

	for _, ifc := range me.Interfaces {
		me.interfaces[strings.ToLower(ifc)] = struct{}{}
	}

	log.Debugf("network io stats will be included for %v", me.interfaces)

	return me, nil
}

func (m *Metric) Collect() error {

	if !m.Enabled {
		return nil
	}

	stats, err := net.IOCounters(true)
	if err != nil {
		return errors.Wrap(err, "network io counters")
	}

	var networkInBytes, networkOutBytes, networkInPackets, networkOutPackets, Errin, Errout, Dropin, Dropout uint64
	for _, counters := range stats {
		if m.interfaces != nil && len(m.interfaces) > 0 {
			name := strings.ToLower(counters.Name)
			if _, include := m.interfaces[name]; !include {
				continue
			}
		}

		if m.Detail {
			event.Save(event.Event{
				Metadata: event.EventMetadata{
					Category: "host",
					Name:     "network",
					Datatype: "accumulate",
					Labels: util.MapStr{
						"ip": util.GetLocalIPs(),
					},
				},
				Fields: ioCountersToMapStr(counters),
			})
		}

		// accumulate values from all interfaces
		networkInBytes += counters.BytesRecv
		networkOutBytes += counters.BytesSent
		networkInPackets += counters.PacketsRecv
		networkOutPackets += counters.PacketsSent
		Errin += counters.Errin
		Errout += counters.Errout
		Dropin += counters.Dropin
		Dropout += counters.Dropout
	}

	if m.Summary {
		if m.prevCounters != (networkCounter{}) {
			// convert network metrics from counters to gauges
			event.Save(event.Event{
				Metadata: event.EventMetadata{
					Category: "host",
					Name:     "network_summary",
					Datatype: "accumulate",
					Labels: util.MapStr{
						"ip": util.GetLocalIPs(),
					},
				},
				Fields: util.MapStr{
					"host": util.MapStr{
						"network_summary": util.MapStr{
							"in.bytes":    networkInBytes,
							"in.packets":  networkInPackets,
							"in.errors":   Errin,
							"in.dropped":  Dropin,
							"out.bytes":   networkOutBytes,
							"out.packets": networkOutPackets,
							"out.errors":  Errout,
							"out.dropped": Dropout,
						},
					},
				},
			})
		}
	}

	if m.Throughput {
		if m.prevCounters != (networkCounter{}) {
			// convert network metrics from counters to gauges
			event.Save(event.Event{
				Metadata: event.EventMetadata{
					Category: "host",
					Name:     "network_throughput",
					Datatype: "gauge",
					Labels: util.MapStr{
						"ip": util.GetLocalIPs(),
					},
				},
				Fields: util.MapStr{
					"host": util.MapStr{
						"network_throughput": util.MapStr{
							"in.bytes":    networkInBytes - m.prevCounters.prevNetworkInBytes,
							"in.packets":  networkInPackets - m.prevCounters.prevNetworkInPackets,
							"out.bytes":   networkOutBytes - m.prevCounters.prevNetworkOutBytes,
							"out.packets": networkOutPackets - m.prevCounters.prevNetworkOutPackets,
						},
					},
				},
			})
		}
	}

	if m.Sockets{
		// all network connections
		conns, err := connections("inet")
		if err != nil {
			return errors.Wrap(err, "error getting connections")
		}

		stats := calculateConnStats(conns)
		stats,_=applyEnhancements(stats)

		if m.prevCounters != (networkCounter{}) {
			event.Save(event.Event{
				Metadata: event.EventMetadata{
					Category: "host",
					Name:     "network_sockets",
					Datatype: "gauge",
					Labels: util.MapStr{
						"ip": util.GetLocalIPs(),
					},
				},
				Fields: util.MapStr{
					"host": util.MapStr{
						"network_sockets": stats,
					},
				},
			})
		}
	}

	//total traffics of all interfaces on host
	// update prevCounters
	m.prevCounters.prevNetworkInBytes = networkInBytes
	m.prevCounters.prevNetworkInPackets = networkInPackets
	m.prevCounters.prevNetworkOutBytes = networkOutBytes
	m.prevCounters.prevNetworkOutPackets = networkOutPackets

	return nil
}

func ioCountersToMapStr(counters net.IOCountersStat) util.MapStr {
	return util.MapStr{
		"host": util.MapStr{
			"network": util.MapStr{
				"name":       counters.Name,
				"in.errors":  counters.Errin,
				"in.dropped": counters.Dropin,
				"in.bytes":   counters.BytesRecv,
				"in.packets": counters.PacketsRecv,

				"out.errors":  counters.Errout,
				"out.dropped": counters.Dropout,
				"out.packets": counters.PacketsSent,
				"out.bytes":   counters.BytesSent,
			},
		}}
}

type SocketStats struct {
	TcpConns       uint `json:"connections,omitempty"`
	TcpListening   uint `json:"listening,omitempty"`
	TcpClosewait   uint `json:"close_wait,omitempty"`
	TcpEstablished uint `json:"established,omitempty"`
	TcpTimewait    uint `json:"time_wait,omitempty"`
	TcpSynsent     uint `json:"sync_sent,omitempty"`
	TcpSynrecv     uint `json:"sync_recv,omitempty"`
	TcpFinwait1    uint `json:"fin_wait1,omitempty"`
	TcpFinwait2    uint `json:"fin_wait2,omitempty"`
	TcpLastack     uint `json:"last_ack,omitempty"`
	TcpClosing     uint `json:"closing,omitempty"`
}

//fetch socks
func calculateConnStats(conns []net.ConnectionStat) util.MapStr {
	var (
		allConns       = len(conns)
		allListening   = 0
		allEstablished = 0
		udpConns       = 0
	)

	ipStats:=map[string]*SocketStats{}

	for _, conn := range conns {
		ip:=conn.Laddr.IP
		if ip==""{
			continue
		}
		s,ok:=ipStats[ip]
		if!ok{
			s=&SocketStats{}
			ipStats[ip]=s
		}

		if conn.Status == "LISTEN" {
			allListening++
		}
		switch conn.Type {
		case syscall.SOCK_STREAM:
			s.TcpConns++
			if conn.Status == "ESTABLISHED" {
				allEstablished++
				s.TcpEstablished++
			}
			if conn.Status == "CLOSE_WAIT" {
				s.TcpClosewait++
			}
			if conn.Status == "TIME_WAIT" {
				s.TcpTimewait++
			}
			if conn.Status == "LISTEN" {
				s.TcpListening++
			}
			if conn.Status == "SYN_SENT" {
				s.TcpSynsent++
			}
			if conn.Status == "SYN_RECV" {
				s.TcpSynrecv++
			}
			if conn.Status == "FIN_WAIT1" {
				s.TcpFinwait1++
			}
			if conn.Status == "FIN_WAIT2" {
				s.TcpFinwait2++
			}
			if conn.Status == "LAST_ACK" {
				s.TcpLastack++
			}
			if conn.Status == "CLOSING" {
				s.TcpClosing++
			}
		case syscall.SOCK_DGRAM:
			udpConns++
		}
	}

	return util.MapStr{
		"all": util.MapStr{
			"connections": allConns,
			"established": allEstablished,
			"listening": allListening,
			"udp": udpConns,
		},
		"tcp": ipStats,
	}
}