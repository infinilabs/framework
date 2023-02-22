/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package network

import (
	"strings"

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

	//total traffics of all interfaces on host
	// update prevCounters
	//m.prevCounters =
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
