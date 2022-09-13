/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package agent

import (
	"encoding/json"
	log "github.com/cihub/seelog"
	"github.com/struCoder/pidusage"
	"infini.sh/framework/core/agent"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/util"
	"os"
	"time"
)

type Metric struct {
	Enabled       bool `config:"enabled"`
	agentBootTime int64
}

func New(cfg *config.Config) (*Metric, error) {

	me := &Metric{
		Enabled: true,
	}

	err := cfg.Unpack(&me)
	if err != nil {
		panic(err)
	}

	return me, nil
}

func (m *Metric) Collect() error {
	var hs []byte
	var err error
	if m.agentBootTime == 0 {
		hs, err = kv.GetValue(agent.KVInstanceBucket, []byte(agent.KVInstanceInfo))
		if err != nil {
			log.Error(err)
			return nil
		}
		if hs == nil {
			return nil
		}
		var instanceInfoMap map[string]interface{}
		err = json.Unmarshal(hs, &instanceInfoMap)
		if err != nil {
			log.Error(err)
			return nil
		}
		value := instanceInfoMap["boot_time"]
		bootTime,ok := value.(float64)
		if !ok || bootTime == 0{
			return nil
		}
		m.agentBootTime = int64(bootTime)
	}

	upTime := time.Now().UnixMilli() - m.agentBootTime
	sysInfo, err := pidusage.GetStat(os.Getpid())
	if err != nil {
		return err
	}
	event.Save(event.Event{
		Metadata: event.EventMetadata{
			Category: "agent",
			Name:     "agent_basic",
			Datatype: "gauge",
		},
		Fields: util.MapStr{
			"agent": util.MapStr{
				"agent_basic": util.MapStr{
					"uptime_in_ms":   upTime,
					"cpu":             sysInfo.CPU,
					"memory_in_bytes": sysInfo.Memory,
					"memory":          util.ByteSize(uint64(sysInfo.Memory)),
				},
			},
		},
	})
	return nil
}