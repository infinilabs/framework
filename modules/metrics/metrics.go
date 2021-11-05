package metrics

import (
	log "github.com/cihub/seelog"
	. "infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/task"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/metrics/elastic"
	"infini.sh/framework/modules/metrics/host/network"
)

type MetricConfig struct {
	MajorIPPattern string `config:"major_ip_pattern"`
	Queue          string `config:"queue"`

	NetworkConfig       *Config `config:"network"`
	ElasticsearchConfig *Config `config:"elasticsearch"`

	Tags   []string          `config:"tags"`
	Labels map[string]string `config:"labels"`
}

type MetricsModule struct {
	config *MetricConfig
}

func (this *MetricsModule) Name() string {
	return "metrics"
}

func (module *MetricsModule) Setup(cfg *Config) {

	module.config = &MetricConfig{}

	_, err := env.ParseConfig("metrics", module.config)
	if err != nil {
		panic(err)
	}

}

func (module *MetricsModule) Start() error {

	_, publicIP, _, _ := util.GetPublishNetworkDeviceInfo(module.config.MajorIPPattern)
	meta := event.AgentMeta{
		MajorIP:   publicIP,
		Hostname:  util.GetHostName(),
		IP:        util.GetLocalIPs(),
		QueueName: util.StringDefault(module.config.Queue, "metrics"),
		Labels:    module.config.Labels,
		Tags:      module.config.Tags}
	event.RegisterMeta(&meta)

	log.Infof("ip:%v, host:%v, labels:%v, tags:%v", meta.MajorIP, meta.Hostname, util.JoinMapString(meta.Labels, "->"), util.JoinArray(meta.Tags, ","))

	//network
	net, err := network.New(module.config.NetworkConfig)
	if err != nil {
		panic(err)
	}
	if net.Enabled{
		var task1 = task.ScheduleTask{
			Description: "fetch network metrics",
			Type:        "schedule",
			Interval:    "10s",
			Task: func() {
				log.Debug("collecting network metrics")
				net.Collect()
			},
		}
		task.RegisterScheduleTask(task1)
	}



	//elasticsearch
	es, err := elastic.New(module.config.ElasticsearchConfig)
	if err != nil {
		panic(err)
	}
	if es.Enabled{
		var task1 = task.ScheduleTask{
			Description: "monitoring for elasticsearch clusters",
			Type:        "interval",
			Interval:    "10s",
			Task: func() {
				log.Debug("collecting elasticsearch metrics")
				es.Collect()
			},
		}
		task.RegisterScheduleTask(task1)
	}

	return nil
}

func (module *MetricsModule) Stop() error {
	return nil
}
