/* ©INFINI, All Rights Reserved.
 * mail: contact#infini.ltd */

package network

import (
	log "github.com/cihub/seelog"
	"github.com/shirou/gopsutil/net"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/util"
	"strings"
)

type Metric struct {
	interfaces   map[string]struct{}
	prevCounters networkCounter

	Enabled    bool     `config:"enabled"`
	Summary    bool     `config:"summary"`
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

	err:=cfg.Unpack(&me)
	if err!=nil{
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

	if !m.Enabled{
		return nil
	}

	stats, err := net.IOCounters(true)
	if err != nil {
		return errors.Wrap(err, "network io counters")
	}

	var networkInBytes, networkOutBytes, networkInPackets, networkOutPackets uint64

	for _, counters := range stats {
		if m.interfaces != nil &&len(m.interfaces)>0{
			name := strings.ToLower(counters.Name)
			if _, include := m.interfaces[name]; !include {
				continue
			}
		}

		if m.Detail {
			event.Save(event.Event{
				Metadata: event.EventMetadata{
					Category: "network",
					Name: "interfaces",
					Datatype: "accumulate",
				},
				Fields: ioCountersToMapStr(counters),
			})
		}

		// accumulate values from all interfaces
		networkInBytes += counters.BytesRecv
		networkOutBytes += counters.BytesSent
		networkInPackets += counters.PacketsRecv
		networkOutPackets += counters.PacketsSent
	}

	if m.Summary {
		if m.prevCounters != (networkCounter{}) {
			// convert network metrics from counters to gauges
			event.Save( event.Event{
				Metadata: event.EventMetadata{
					Category: "network",
					Name: "summary",
					Datatype: "gauge",
				},
				Fields: util.MapStr{
					"network": util.MapStr{
						"total": util.MapStr{
							"in": util.MapStr{
								"bytes":   networkInBytes - m.prevCounters.prevNetworkInBytes,
								"packets": networkInPackets - m.prevCounters.prevNetworkInPackets,
							},
							"out": util.MapStr{
								"bytes":   networkOutBytes - m.prevCounters.prevNetworkOutBytes,
								"packets": networkOutPackets - m.prevCounters.prevNetworkOutPackets,
							},
						},
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
		"network": util.MapStr{
			"interface": util.MapStr{
				"name": counters.Name,
				"in": util.MapStr{
					"errors":  counters.Errin,
					"dropped": counters.Dropin,
					"bytes":   counters.BytesRecv,
					"packets": counters.PacketsRecv,
				},
				"out": util.MapStr{
					"errors":  counters.Errout,
					"dropped": counters.Dropout,
					"packets": counters.PacketsSent,
					"bytes":   counters.BytesSent,
				},
			}}}
}
