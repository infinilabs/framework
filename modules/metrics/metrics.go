package metrics

import (
	"context"
	"encoding/json"
	"fmt"
	log "github.com/cihub/seelog"
	"infini.sh/framework/core/agent"
	. "infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/kv"
	"infini.sh/framework/core/task"
	"infini.sh/framework/core/util"
	agent2 "infini.sh/framework/modules/metrics/agent"
	"infini.sh/framework/modules/metrics/elastic"
	"infini.sh/framework/modules/metrics/host/cpu"
	"infini.sh/framework/modules/metrics/host/disk"
	"infini.sh/framework/modules/metrics/host/memory"
	"infini.sh/framework/modules/metrics/host/network"
	"sync"
)

type MetricConfig struct {
	Enabled bool  `config:"enabled"`
	MajorIPPattern string `config:"major_ip_pattern"`
	Queue          string `config:"queue"`

	NetworkConfig        *Config `config:"network"`
	DiskConfig           *Config `config:"disk"`
	CPUConfig            *Config `config:"cpu"`
	MemoryConfig         *Config `config:"memory"`
	AgentConfig          *Config `config:"agent"`
	ElasticsearchConfig  *Config `config:"elasticsearch"`

	Tags   []string          `config:"tags"`
	Labels map[string]string `config:"labels"`

	agentId       string
	hostId        string
	hostMetricMux sync.Mutex
}

type MetricsModule struct {
	config *MetricConfig
}

func (this *MetricsModule) Name() string {
	return "metrics"
}

func (module *MetricsModule) Setup() {

	module.config = &MetricConfig{Enabled: true}

	exits, err := env.ParseConfig("metrics", module.config)
	if !exits{
		module.config.Enabled=false
		return
	}
	if err != nil {
		panic(err)
	}

	if !module.config.Enabled{
		return
	}


	_, publicIP, _, _ := util.GetPublishNetworkDeviceInfo(module.config.MajorIPPattern)
	meta := event.AgentMeta{
		MajorIP:   publicIP,
		Hostname:  util.GetHostName(),
		IP:        util.GetLocalIPs(),
		QueueName: util.StringDefault(module.config.Queue, "metrics"),
		Labels:    module.config.Labels,
		Tags:      module.config.Tags}

	event.RegisterMeta(&meta)

	tail:=fmt.Sprintf("ip: %v,host: %v", meta.MajorIP, meta.Hostname)
	if len(meta.Labels)>0{
		tail=tail+",labels: "+util.JoinMapString(meta.Labels, "->")
	}
	if len(meta.Tags)>0{
		tail=tail+",tags: "+util.JoinArray(meta.Tags, ",")
	}

	log.Info(tail)

	CollectAgentMetric(module)
	CollectHostMetric(module)
	CollectESMetric(module)
}

func CollectESMetric(module *MetricsModule){
	if module.config.ElasticsearchConfig!=nil{
		//elasticsearch
		es, err := elastic.New(module.config.ElasticsearchConfig)
		if err != nil {
			panic(err)
		}
		if es.Enabled{
			var task1 = task.ScheduleTask{
				Description: "monitoring for elasticsearch clusters",
				Type:        "interval",
				Interval:    es.Interval,
				Task: func(ctx context.Context) {
					log.Debug("collecting elasticsearch metrics")
					es.Collect()
				},
			}
			task.RegisterScheduleTask(task1)
		}
	}
}

func CollectAgentMetric(module *MetricsModule) {
	if module.config.AgentConfig != nil{
		agentM, err := agent2.New(module.config.AgentConfig)
		if err != nil {
			panic(err)
		}
		if agentM.Enabled{
			var task1 = task.ScheduleTask{
				Description: "fetch agent metrics",
				Type:        "interval",
				Interval:    "10s",
				Task: func(ctx context.Context) {
					log.Debug("collecting agent metrics")
					if module.isReady() {
						agentM.Collect()
					}
				},
			}
			task.RegisterScheduleTask(task1)
		}
	}
}

func CollectHostMetric(module *MetricsModule)  {
	var netM *network.Metric
	var diskM *disk.Metric
	var cpuM *cpu.Metric
	var memoryM *memory.Metric
	var err error

	if module.config.NetworkConfig != nil{
		if netM == nil {
			netM, err = network.New(module.config.NetworkConfig)
			if err != nil {
				panic(err)
			}
		}
		var netTask = task.ScheduleTask{
			Description: "fetch network metrics",
			Type:        "interval",
			Interval:    "10s",
			Task: func(ctx context.Context) {
				if !module.isReady() {
					return
				}
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
		var diskTask = task.ScheduleTask{
			Description: "fetch disk metrics",
			Type:        "interval",
			Interval:    "10s",
			Task: func(ctx context.Context) {
				if !module.isReady() {
					return
				}
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
		var cpuTask = task.ScheduleTask{
			Description: "fetch cpu metrics",
			Type:        "interval",
			Interval:    "10s",
			Task: func(ctx context.Context) {
				if !module.isReady() {
					return
				}
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
		var memTask = task.ScheduleTask{
			Description: "fetch memory metrics",
			Type:        "interval",
			Interval:    "10s",
			Task: func(ctx context.Context) {
				if !module.isReady() {
					return
				}
				log.Debug("collecting memory metrics")
				memoryM.Collect()
			},
		}
		task.RegisterScheduleTask(memTask)
	}
}

func (module *MetricsModule) Start() error {

	return nil
}

func (module *MetricsModule) Stop() error {

	//TODO cancel or stop background jobs

	return nil
}

func (module *MetricsModule) isReady() bool {
	module.config.hostMetricMux.Lock()
	defer module.config.hostMetricMux.Unlock()
	if module.config.agentId != "" && module.config.hostId != "" {
		return true
	}
	hs, err := kv.GetValue(agent.KVInstanceBucket, []byte(agent.KVInstanceInfo))
	if err != nil {
		log.Error(err)
		return false
	}
	if hs == nil {
		return false
	}
	var instanceInfoMap map[string]interface{}
	err = json.Unmarshal(hs, &instanceInfoMap)
	if err != nil {
		log.Errorf("config.getInstanceInfoFromKV: %v\n", err)
		return false
	}
	agentId := instanceInfoMap["agent_id"]
	hostID := instanceInfoMap["host_id"]
	if hostID == nil || agentId == nil || agentId =="" || hostID == "" {
		return false
	}
	module.config.agentId = agentId.(string)
	module.config.hostId = hostID.(string)
	return true
}
