/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package pipeline

import (
	"net/http"
	"time"

	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/pipeline"
)

func (h *PipeModule) createPipelineHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var obj = &pipeline.PipelineConfigV2{}
	err := h.DecodeJSON(req, obj)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if obj.Name == "" {
		h.WriteError(w, "name is required", http.StatusBadRequest)
		return
	}
	// Use name as the document ID so that the pipeline name is the single
	// identifier across the URL, ES _id, and the in-memory task map.
	obj.ID = obj.Name

	ctx := orm.NewContextWithParent(req.Context())
	ctx.Refresh = orm.WaitForRefresh

	err = orm.Create(ctx, obj)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.WriteCreatedOKJSON(w, obj.ID)
}

func (h *PipeModule) getPipelineHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")

	obj := pipeline.PipelineConfigV2{}
	obj.ID = id

	ctx := orm.NewContextWithParent(req.Context())
	exists, err := orm.GetV2(ctx, &obj)
	if !exists || err != nil {
		h.WriteOpRecordNotFoundJSON(w, id)
		return
	}

	h.WriteGetOKJSON(w, id, obj)
}

func (h *PipeModule) updatePipelineHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	replace := h.GetBoolOrDefault(req, "replace", false)
	var newConfig pipeline.PipelineConfigV2
	err := h.DecodeJSON(req, &newConfig)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if newConfig.ID != "" && newConfig.ID != id {
		h.WriteError(w, "id in request body does not match id in URL", http.StatusBadRequest)
		return
	}

	if newConfig.Name != "" && newConfig.Name != id {
		h.WriteError(w, "pipeline name cannot be changed as it is the pipeline's unique identifier", http.StatusBadRequest)
		return
	}

	ctx := orm.NewContextWithParent(req.Context())

	if !replace {
		var oldConfig pipeline.PipelineConfigV2
		oldConfig.ID = id

		exists, err := orm.GetWithSystemFields(ctx, &oldConfig)
		if !exists || err != nil {
			h.WriteOpRecordNotFoundJSON(w, id)
			return
		}

		newConfig.Created = oldConfig.Created
	} else {
		t := time.Now()
		newConfig.Created = &t
	}

	// Ensure ID is set even if the client omitted it from the request body
	newConfig.ID = id

	ctx.Refresh = orm.WaitForRefresh
	err = orm.Save(ctx, &newConfig)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.WriteUpdatedOKJSON(w, newConfig.ID)
}

func (h *PipeModule) deletePipelineHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")

	obj := pipeline.PipelineConfigV2{}
	obj.ID = id

	ctx := orm.NewContextWithParent(req.Context())

	exists, err := orm.GetV2(ctx, &obj)
	if !exists || err != nil {
		h.WriteOpRecordNotFoundJSON(w, id)
		return
	}

	ctx.Refresh = orm.WaitForRefresh
	err = orm.Delete(ctx, &obj)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.WriteDeletedOKJSON(w, id)
}

func (h *PipeModule) searchPipelineHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {

	var err error
	builder, err := orm.NewQueryBuilderFromRequest(req, "name", "combined_fulltext")
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(builder.Sorts()) == 0 {
		builder.SortBy(orm.Sort{Field: "created", SortType: orm.DESC})
	}

	builder.EnableBodyBytes()

	ctx := orm.NewContextWithParent(req.Context())
	orm.WithModel(ctx, &pipeline.PipelineConfigV2{})

	var objs []pipeline.PipelineConfigV2

	err, res := elastic.SearchV2WithResultItemMapper(ctx, &objs, builder, nil)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_, err = h.Write(w, res.Raw)
	if err != nil {
		h.Error(w, err)
	}
}
