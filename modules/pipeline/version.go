/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	log "github.com/cihub/seelog"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/pipeline"
	"infini.sh/framework/core/util"
)

// PipelineVersion is the immutable snapshot of one pipeline config at a
// point in time. Every PUT /pipelines/:id auto-snapshots the previous
// state before overwriting — no explicit "publish" step; every edit is a
// version.
type PipelineVersion struct {
	orm.ORMObjectBase `json:",inline" elastic_mapping:"-"`

	PipelineID string `json:"pipeline_id" elastic_mapping:"pipeline_id:{type:keyword}"`
	Version    int    `json:"version" elastic_mapping:"version:{type:integer}"`
	// Config is the full PipelineConfigV2 JSON snapshot.
	Config string `json:"config" elastic_mapping:"config:{type:text}"`
	Note   string `json:"note,omitempty" elastic_mapping:"note:{type:text}"`
	// UpdatedBy records who made the change (from the request context).
	UpdatedBy string    `json:"updated_by,omitempty" elastic_mapping:"updated_by:{type:keyword}"`
	CreatedAt time.Time `json:"created_at" elastic_mapping:"created_at:{type:date}"`
}

// maxVersionsPerPipeline bounds the retention (oldest pruned on write).
const maxVersionsPerPipeline = 50

// saveVersion snapshots the current config before an update lands.
// Transient pipelines skip versioning — they are ad-hoc executions that
// produce no durable config worth tracking (saving them wastes storage).
func (h *PipeModule) saveVersion(octx *orm.Context, current *pipeline.PipelineConfigV2, note string) error {
	if current.Transient {
		return nil // transient: no version history
	}
	// find the highest existing version number
	next := h.nextVersion(octx, current.ID) + 1
	cfgJSON := string(util.MustToJSONBytes(current))

	v := PipelineVersion{
		PipelineID: current.ID,
		Version:    next,
		Config:     cfgJSON,
		Note:       note,
		CreatedAt:  time.Now().UTC(),
	}
	v.ID = fmt.Sprintf("%s-v%d", current.ID, next)

	if err := orm.Save(octx, &v); err != nil {
		return fmt.Errorf("save version snapshot: %w", err)
	}

	// prune oldest beyond retention (best effort — failure is a warning)
	go h.pruneVersions(current.ID)
	return nil
}

func (h *PipeModule) nextVersion(octx *orm.Context, pipelineID string) int {
	orm.WithModel(octx, &PipelineVersion{})
	q := orm.NewQuery().
		Filter(orm.TermQuery("pipeline_id", pipelineID)).
		SortBy(orm.Sort{Field: "version", SortType: orm.DESC}).Size(1)
	res, err := orm.SearchV2(octx, q)
	if err != nil || res == nil {
		return 0
	}
	versions, _, _ := decodeVersions(res)
	if len(versions) == 0 {
		return 0
	}
	return versions[0].Version
}

func (h *PipeModule) pruneVersions(pipelineID string) {
	octx := orm.NewContext().DirectAccess()
	orm.WithModel(octx, &PipelineVersion{})
	q := orm.NewQuery().
		Filter(orm.TermQuery("pipeline_id", pipelineID)).
		SortBy(orm.Sort{Field: "version", SortType: orm.DESC}).Size(maxVersionsPerPipeline + 10)
	res, err := orm.SearchV2(octx, q)
	if err != nil {
		return
	}
	versions, _, _ := decodeVersions(res)
	if len(versions) <= maxVersionsPerPipeline {
		return
	}
	for _, old := range versions[maxVersionsPerPipeline:] {
		_ = orm.Delete(octx, &old)
	}
}

func decodeVersions(res *orm.SearchResult) ([]PipelineVersion, int64, error) {
	return elastic.DecodeHits[PipelineVersion](res)
}

// ─── endpoints ────────────────────────────────────────────────────────────

// listPipelineVersionsHandler — GET /pipelines/:id/versions
func (h *PipeModule) listPipelineVersionsHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	octx := orm.NewContextWithParent(req.Context()).DirectAccess()
	orm.WithModel(octx, &PipelineVersion{})
	q := orm.NewQuery().
		Filter(orm.TermQuery("pipeline_id", id)).
		SortBy(orm.Sort{Field: "version", SortType: orm.DESC}).Size(50)
	res, err := orm.SearchV2(octx, q)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	versions, total, _ := decodeVersions(res)
	// strip the full config from the list view (it can be large)
	type versionSummary struct {
		Version   int       `json:"version"`
		Note      string    `json:"note,omitempty"`
		UpdatedBy string    `json:"updated_by,omitempty"`
		CreatedAt time.Time `json:"created_at"`
	}
	out := make([]versionSummary, 0, len(versions))
	for _, v := range versions {
		out = append(out, versionSummary{v.Version, v.Note, v.UpdatedBy, v.CreatedAt})
	}
	h.WriteJSON(w, util.MapStr{"total": total, "versions": out}, http.StatusOK)
}

// getPipelineVersionHandler — GET /pipelines/:id/versions/:v
func (h *PipeModule) getPipelineVersionHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	vStr := ps.MustGetParameter("version")
	want, err := strconv.Atoi(vStr)
	if err != nil {
		h.WriteError(w, "invalid version number", http.StatusBadRequest)
		return
	}
	v := h.findVersion(req, id, want)
	if v == nil {
		h.WriteOpRecordNotFoundJSON(w, vStr)
		return
	}
	h.WriteJSON(w, v, http.StatusOK)
}

// rollbackPipelineHandler — POST /pipelines/:id/versions/:v/rollback
// Restores the config from the target version snapshot and writes a new
// version entry recording the rollback.
func (h *PipeModule) rollbackPipelineHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	vStr := ps.MustGetParameter("version")
	want, err := strconv.Atoi(vStr)
	if err != nil {
		h.WriteError(w, "invalid version number", http.StatusBadRequest)
		return
	}

	target := h.findVersion(req, id, want)
	if target == nil {
		h.WriteOpRecordNotFoundJSON(w, vStr)
		return
	}

	// snapshot the CURRENT config before rollback (so the rollback itself
	// is also revertible)
	octx := orm.NewContextWithParent(req.Context()).DirectAccess()
	current := pipeline.PipelineConfigV2{}
	current.ID = id
	exists, err := orm.GetWithSystemFields(octx, &current)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if exists {
		if err := h.saveVersion(octx, &current, "pre-rollback snapshot"); err != nil {
			log.Warnf("pipeline %s: pre-rollback snapshot failed: %v", id, err)
		}
	}

	// restore the target config
	restored := pipeline.PipelineConfigV2{}
	if err := util.FromJSONBytes([]byte(target.Config), &restored); err != nil {
		h.WriteError(w, fmt.Sprintf("decode version %d config: %v", want, err), http.StatusInternalServerError)
		return
	}
	restored.ID = id // preserve identity

	// compile gate: an old version may reference processors that were
	// renamed or removed since — restoring it must not break the pipeline
	if !h.validateAndReject(w, &restored) {
		return
	}
	octx2 := orm.NewContextWithParent(req.Context())
	octx2.Refresh = orm.WaitForRefresh
	if err := orm.Save(octx2, &restored); err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.WriteJSON(w, util.MapStr{
		"rolled_back_to": want,
		"id":             id,
		"note":           target.Note,
	}, http.StatusOK)
}

func (h *PipeModule) findVersion(req *http.Request, pipelineID string, version int) *PipelineVersion {
	octx := orm.NewContextWithParent(req.Context()).DirectAccess()
	orm.WithModel(octx, &PipelineVersion{})
	q := orm.NewQuery().
		Filter(orm.TermQuery("pipeline_id", pipelineID)).
		Filter(orm.TermQuery("version", version)).Size(1)
	res, err := orm.SearchV2(octx, q)
	if err != nil {
		return nil
	}
	versions, _, _ := decodeVersions(res)
	if len(versions) == 0 {
		return nil
	}
	return &versions[0]
}
