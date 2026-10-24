/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"net/http"
	"sort"
	"time"

	log "github.com/cihub/seelog"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
)

// PipelineStageStat is the persisted snapshot row of one stage at one
// sampling tick (D-7: framework built-in persistence — one exporter,
// every product inherits).
type PipelineStageStat struct {
	orm.ORMObjectBase `json:",inline" elastic_mapping:"-"`

	// InstanceID identifies the emitting node — cross-instance queries
	// group on it; the #130 acceptance (cluster aggregate = sum of the
	// per-instance rows) keys off this column.
	InstanceID string    `json:"instance_id" elastic_mapping:"instance_id:{type:keyword}"`
	Pipeline   string    `json:"pipeline" elastic_mapping:"pipeline:{type:keyword}"`
	Stage      string    `json:"stage" elastic_mapping:"stage:{type:keyword}"`
	Carrier    string    `json:"carrier" elastic_mapping:"carrier:{type:keyword}"`
	Count      int64     `json:"count" elastic_mapping:"count:{type:long}"`
	Errors     int64     `json:"errors" elastic_mapping:"errors:{type:long}"`
	AvgLatency int64     `json:"avg_latency_ns" elastic_mapping:"avg_latency_ns:{type:long}"`
	Ts         time.Time `json:"ts" elastic_mapping:"ts:{type:date}"`
}

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
	// DirectAccess: this is a background system writer (no request user);
	// the security orm hooks require either a session user or the direct
	// read/write flags — a bare context panics on the first save under
	// security-enabled apps (coco) and takes the process down
	octx := orm.NewContext().DirectAccess()
	for _, name := range names {
		for _, st := range stageStats(name) {
			if st.Count == 0 && st.Errors == 0 {
				continue
			}
			row := PipelineStageStat{
				InstanceID: st.InstanceID, Pipeline: name, Stage: st.Stage, Carrier: st.Carrier,
				Count: st.Count, Errors: st.Errors, AvgLatency: st.AvgLatency,
				Ts: now,
			}
			row.ID = util.GetUUID()
			if err := orm.Save(octx, &row); err != nil {
				log.Warnf("metrics persist: save %s/%s failed: %v", name, st.Stage, err)
			}
		}
	}
}

// ClusterStageStat is one stage aggregated across every instance that
// reported it: counters sum over each instance's LATEST snapshot tick,
// latency is the invocation-weighted mean, and PerInstance carries the
// detail rows so the cluster math stays auditable (aggregate must equal
// the sum of the parts).
type ClusterStageStat struct {
	Pipeline    string                `json:"pipeline"`
	Stage       string                `json:"stage"`
	Carrier     string                `json:"carrier"`
	Count       int64                 `json:"count"`
	Errors      int64                 `json:"errors"`
	AvgLatency  int64                 `json:"avg_latency_ns"`
	PerInstance map[string]StageStats `json:"per_instance"`
}

// aggregateStageRows folds persisted snapshot rows (cumulative counters
// per tick) into per-stage cluster totals. Rows are sorted newest-first
// internally; the first row of each (instance, stage) pair is that
// instance's latest tick — older ticks are superseded, not summed.
func aggregateStageRows(rows []PipelineStageStat) []ClusterStageStat {
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Ts.After(rows[j].Ts) })

	type stageKey struct{ pipeline, stage, carrier string }
	latest := map[string]map[stageKey]StageStats{} // instance -> stage -> row
	for _, r := range rows {
		inst := r.InstanceID
		if inst == "" {
			inst = "?" // pre-#127 rows: attribute to a synthetic bucket
		}
		key := stageKey{r.Pipeline, r.Stage, r.Carrier}
		m := latest[inst]
		if m == nil {
			m = map[stageKey]StageStats{}
			latest[inst] = m
		}
		if _, seen := m[key]; seen {
			continue
		}
		m[key] = StageStats{
			InstanceID: r.InstanceID, Stage: r.Stage, Carrier: r.Carrier,
			Count: r.Count, Errors: r.Errors, AvgLatency: r.AvgLatency,
		}
	}

	totals := map[stageKey]*ClusterStageStat{}
	for _, m := range latest {
		for key, st := range m {
			agg := totals[key]
			if agg == nil {
				agg = &ClusterStageStat{
					Pipeline: key.pipeline, Stage: key.stage, Carrier: key.carrier,
					PerInstance: map[string]StageStats{},
				}
				totals[key] = agg
			}
			agg.Count += st.Count
			agg.Errors += st.Errors
			agg.PerInstance[st.InstanceID] = st
		}
	}
	out := make([]ClusterStageStat, 0, len(totals))
	for _, agg := range totals {
		var totalInvocations int64
		for _, st := range agg.PerInstance {
			totalInvocations += st.Count
		}
		if totalInvocations > 0 {
			var weighted int64
			for _, st := range agg.PerInstance {
				weighted += st.AvgLatency * st.Count
			}
			agg.AvgLatency = weighted / totalInvocations
		}
		out = append(out, *agg)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Pipeline != out[j].Pipeline {
			return out[i].Pipeline < out[j].Pipeline
		}
		return out[i].Stage < out[j].Stage
	})
	return out
}

// clusterStagesHandler — GET /pipeline/cluster/stages[?pipeline=]
// Cluster-wide stage totals from the persisted snapshots: every
// instance's latest tick summed per stage, with the per-instance detail
// alongside (the aggregate must equal the sum of the parts).
func (h *PipeModule) clusterStagesHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	octx := orm.NewContextWithParent(req.Context()).DirectAccess()
	orm.WithModel(octx, &PipelineStageStat{})
	q := orm.NewQuery().Size(4000)
	if p := req.URL.Query().Get("pipeline"); p != "" {
		q = q.Filter(orm.TermQuery("pipeline", p))
	}
	res, err := orm.SearchV2(octx, q)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	rows, _, _ := elastic.DecodeHits[PipelineStageStat](res)
	h.WriteJSON(w, util.MapStr{"stages": aggregateStageRows(rows)}, http.StatusOK)
}
