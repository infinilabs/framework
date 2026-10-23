/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"sync/atomic"
	"time"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
)

// PipelineStageStat is the persisted snapshot row of one stage at one
// sampling tick (D-7: framework built-in persistence — one exporter,
// every product inherits).
type PipelineStageStat struct {
	orm.ORMObjectBase `json:",inline" elastic_mapping:"-"`

	Pipeline   string    `json:"pipeline" elastic_mapping:"pipeline:{type:keyword}"`
	Stage      string    `json:"stage" elastic_mapping:"stage:{type:keyword}"`
	Carrier    string    `json:"carrier" elastic_mapping:"carrier:{type:keyword}"`
	Count      int64     `json:"count" elastic_mapping:"count:{type:long}"`
	Errors     int64     `json:"errors" elastic_mapping:"errors:{type:long}"`
	AvgLatency int64     `json:"avg_latency_ns" elastic_mapping:"avg_latency_ns:{type:long}"`
	Ts         time.Time `json:"ts" elastic_mapping:"ts:{type:date}"`
}

// metricsPersistFailures counts stage-snapshot rows the exporter failed
// to persist (orm save errors) — cumulative since process start. Losses
// here mean the persisted metric history has holes; surfaced via the
// stages endpoint next to the in-memory stats.
var metricsPersistFailures atomic.Int64

// metricsPersistConfig reads pipeline_metrics.persistence.
type metricsPersistConfig struct {
	Persistence struct {
		Enabled  bool `config:"enabled"`
		Interval int  `config:"interval_seconds"`
	} `config:"persistence"`
}

// startMetricsPersist runs the snapshot exporter when enabled
// (default interval 10s). Snapshots are cumulative counters; consumers
// derive rates by differencing consecutive ticks.
func (module *PipeModule) startMetricsPersist() {
	cfg := metricsPersistConfig{}
	exist, err := env.ParseConfig("pipeline_metrics", &cfg)
	if err != nil || !exist || !cfg.Persistence.Enabled {
		return
	}
	interval := time.Duration(cfg.Persistence.Interval) * time.Second
	if interval <= 0 {
		interval = 10 * time.Second
	}
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if module.closed.Load() {
					return
				}
				module.persistStageSnapshot()
			}
		}
	}()
	log.Debugf("pipeline metrics persistence enabled (interval %v)", interval)
}

func (module *PipeModule) persistStageSnapshot() {
	names := []string{}
	module.contexts.Range(func(k, v interface{}) bool {
		if name, ok := k.(string); ok {
			names = append(names, name)
		}
		return true
	})
	now := time.Now().UTC()
	octx := orm.NewContext()
	for _, name := range names {
		for _, st := range stageStats(name) {
			if st.Count == 0 && st.Errors == 0 {
				continue
			}
			row := PipelineStageStat{
				Pipeline: name, Stage: st.Stage, Carrier: st.Carrier,
				Count: st.Count, Errors: st.Errors, AvgLatency: st.AvgLatency,
				Ts: now,
			}
			row.ID = util.GetUUID()
			if err := orm.Save(octx, &row); err != nil {
				metricsPersistFailures.Add(1)
				log.Warnf("metrics persist: save %s/%s failed: %v", name, st.Stage, err)
			}
		}
	}
}
