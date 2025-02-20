// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package metrics

import (
	"context"
	"fmt"
	log "github.com/cihub/seelog"
	. "infini.sh/framework/core/config"
	elastic2 "infini.sh/framework/core/elastic"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/task"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/metrics/elastic"
	"infini.sh/framework/modules/metrics/host/cpu"
	"infini.sh/framework/modules/metrics/host/disk"
	"infini.sh/framework/modules/metrics/host/memory"
	"infini.sh/framework/modules/metrics/host/network"
	agent2 "infini.sh/framework/modules/metrics/instance"
)

type MetricConfig struct {
	Enabled      bool   `config:"enabled"`
	MetricQueue  string `config:"queue"`
	LoggingQueue string `config:"logging_queue"`

	EventQueue map[string]string `config:"event_queue"` // metadata.name -> queue name

	InstanceConfig *Config `config:"instance"`

	NetworkConfig       *Config `config:"network"`
	DiskConfig          *Config `config:"disk"`
	CPUConfig           *Config `config:"cpu"`
	MemoryConfig        *Config `config:"memory"`
	ElasticsearchConfig *Config `config:"elasticsearch"`

	Tags   []string          `config:"tags"`
	Labels map[string]string `config:"labels"`
}

type MetricsModule struct {
	config   *MetricConfig
	taskIDs  []string
	agent    *event.AgentMeta
	esMetric *elastic.ElasticsearchMetric
}

func (module *MetricsModule) Name() string {
	return "metrics"
}

func (module *MetricsModule) loadConfig(cfg *MetricConfig) {

	meta := module.buildAgentMeta()

	event.RegisterMeta(&meta)
	module.agent = &meta

	tail := fmt.Sprintf("ip: %v,host: %v", meta.MajorIP, meta.Hostname)
	if len(meta.Labels) > 0 {
		tail = tail + ",labels: " + util.JoinMapString(meta.Labels, "->")
	}
	if len(meta.Tags) > 0 {
		tail = tail + ",tags: " + util.JoinArray(meta.Tags, ",")
	}

	module.CollectAgentMetric()
	module.CollectHostMetric()
	module.CollectESMetric()
}
func (module *MetricsModule) Setup() {

	module.config = &MetricConfig{Enabled: true}

	exits, err := env.ParseConfig("metrics", module.config)
	if !exits {
		module.config.Enabled = false
		return
	}
	if err != nil {
		panic(err)
	}
	//register elastic metadata change event callback to handle refresh logic of elastic metric collect tasks
	elastic2.RegisterMetadataChangeEvent(func(meta, oldMeta *elastic2.ElasticsearchMetadata, action elastic2.EventAction) {
		if action == elastic2.EventActionUpdate && oldMeta != nil {
			//skip if no monitor config changed
			hasChanged := false
			changelog, _ := util.DiffTwoObject(oldMeta.Config.MonitorConfigs, meta.Config.MonitorConfigs)
			if len(changelog) > 0 {
				hasChanged = true
			} else if meta != nil {
				// check other conditions
				hasChanged = meta.IsAvailable() != oldMeta.IsAvailable() ||
					meta.Config.Enabled != oldMeta.Config.Enabled ||
					meta.Config.Monitored != oldMeta.Config.Monitored
			}
			if !hasChanged {
				return
			}
		}
		switch action {
		case elastic2.EventActionCreate, elastic2.EventActionUpdate:
			if meta == nil {
				log.Warnf("elastic metadata is nil")
				return
			}
			if module.esMetric != nil {
				module.esMetric.InitialCollectTask(meta.Config.ID, meta)
			}
		case elastic2.EventActionDelete:
			if oldMeta == nil {
				log.Warnf("elastic metadata is nil")
				return
			}
			if module.esMetric != nil {
				module.esMetric.RemoveTasksByClusterID(oldMeta.Config.ID)
			}
		}
	})
}

func (module *MetricsModule) CollectESMetric() {
	if module.config.ElasticsearchConfig != nil {
		//elasticsearch
		es, err := elastic.New(module.config.ElasticsearchConfig, module.onSaveEvent)
		if err != nil {
			panic(err)
		}
		module.esMetric = es
		if es.Enabled {
			es.Collect()
		}
	}
}

func (module *MetricsModule) CollectAgentMetric() {
	if module.config.InstanceConfig != nil {
		agentM, err := agent2.New(module.config.InstanceConfig)
		if err != nil {
			panic(err)
		}
		if agentM.Enabled {
			taskId := util.GetUUID()
			module.taskIDs = append(module.taskIDs, taskId)
			var task1 = task.ScheduleTask{
				ID:          taskId,
				Description: "fetch instance metrics",
				Type:        "interval",
				Interval:    "10s",
				Task: func(ctx context.Context) {
					log.Debug("collecting instance metrics")
					agentM.Collect()
				},
			}
			task.RegisterScheduleTask(task1)
		}
	}
}

func (module *MetricsModule) CollectHostMetric() {
	var netM *network.Metric
	var diskM *disk.Metric
	var cpuM *cpu.Metric
	var memoryM *memory.Metric
	var err error

	if module.config.NetworkConfig != nil {
		if netM == nil {
			netM, err = network.New(module.config.NetworkConfig)
			if err != nil {
				panic(err)
			}
		}
		taskId := util.GetUUID()
		module.taskIDs = append(module.taskIDs, taskId)
		var netTask = task.ScheduleTask{
			ID:          taskId,
			Description: "fetch network metrics",
			Type:        "interval",
			Interval:    "10s",
			Task: func(ctx context.Context) {
				log.Debug("collecting network metrics")
				netM.Collect()
			},
		}
		task.RegisterScheduleTask(netTask)
	}

	if module.config.DiskConfig != nil {
		if diskM == nil {
			diskM, err = disk.New(module.config.DiskConfig)
			if err != nil {
				panic(err)
			}
		}
		taskId := util.GetUUID()
		module.taskIDs = append(module.taskIDs, taskId)
		var diskTask = task.ScheduleTask{
			ID:          taskId,
			Description: "fetch disk metrics",
			Type:        "interval",
			Interval:    "10s",
			Task: func(ctx context.Context) {
				log.Debug("collecting disk metrics")
				diskM.Collect()
			},
		}
		task.RegisterScheduleTask(diskTask)
	}

	if module.config.CPUConfig != nil {
		if cpuM == nil {
			cpuM, err = cpu.New(module.config.CPUConfig)
			if err != nil {
				panic(err)
			}
		}
		taskId := util.GetUUID()
		module.taskIDs = append(module.taskIDs, taskId)
		var cpuTask = task.ScheduleTask{
			ID:          taskId,
			Description: "fetch cpu metrics",
			Type:        "interval",
			Interval:    "10s",
			Task: func(ctx context.Context) {
				log.Debug("collecting cpu metrics")
				cpuM.Collect()
			},
		}
		task.RegisterScheduleTask(cpuTask)
	}

	if module.config.MemoryConfig != nil {
		if memoryM == nil {
			memoryM, err = memory.New(module.config.MemoryConfig)
			if err != nil {
				panic(err)
			}
		}
		taskId := util.GetUUID()
		module.taskIDs = append(module.taskIDs, taskId)
		var memTask = task.ScheduleTask{
			ID:          taskId,
			Description: "fetch memory metrics",
			Type:        "interval",
			Interval:    "10s",
			Task: func(ctx context.Context) {
				log.Debug("collecting memory metrics")
				memoryM.Collect()
			},
		}
		task.RegisterScheduleTask(memTask)
	}
}

func (module *MetricsModule) Start() error {
	if !module.config.Enabled {
		return nil
	}
	module.loadConfig(module.config)
	NotifyOnConfigSectionChange("metrics", func(pCfg, cCfg *Config) {

		if cCfg == nil {
			return
		}

		newCfg := &MetricConfig{}
		err := cCfg.Unpack(newCfg)
		if err != nil || newCfg == nil {
			log.Error(err)
			return
		}

		for _, taskId := range module.taskIDs {
			task.StopTask(taskId)
			task.DeleteTask(taskId)
		}
		if module.esMetric != nil {
			module.esMetric.RemoveAllCollectTasks()
		}
		module.taskIDs = nil

		module.config = newCfg

		module.loadConfig(module.config)

	})

	return nil
}

func (m *MetricsModule) onSaveEvent(item *event.Event) error {
	log.Tracef("event queue name: %v, meta: %v", m.config.EventQueue, item.Metadata.Name)
	if m.config.EventQueue != nil {
		if v, ok := m.config.EventQueue[item.Metadata.Name]; ok {
			if v != "" {
				item.QueueName = v
			}
		}
	}
	item.Agent = m.agent
	return event.Save(item)
}
func (module *MetricsModule) Stop() error {

	//TODO cancel or stop background jobs

	return nil
}

func (module *MetricsModule) buildAgentMeta() event.AgentMeta {
	labels := map[string]string{}
	for k, v := range global.Env().SystemConfig.NodeConfig.Labels {
		labels[k] = v
	}
	for k, v := range module.config.Labels {
		labels[k] = v
	}
	_, publicIP, _, _ := util.GetPublishNetworkDeviceInfo(global.Env().SystemConfig.NodeConfig.MajorIpPattern)
	return event.AgentMeta{
		AgentID:                global.Env().SystemConfig.NodeConfig.ID,
		MajorIP:                publicIP,
		Hostname:               util.GetHostName(),
		IP:                     util.GetLocalIPs(),
		DefaultMetricQueueName: util.StringDefault(module.config.MetricQueue, "metrics"),
		LoggingQueueName:       util.StringDefault(module.config.LoggingQueue, "logging"),
		Labels:                 labels,
		Tags:                   module.config.Tags,
	}
}
