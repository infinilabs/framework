/* Copyright © INFINI LTD. All rights reserved. */

// Package crud generates the standard five-piece REST CRUD (create / search /
// get / update / delete) for an ORM model — the scaffold coco's modules and
// modules/easysearch each hand-copy today.
//
// The generated handlers are the P1/P6 conventions made concrete:
//   - search: orm.NewQueryBuilderFromRequest + EnableBodyBytes, default sort
//     created DESC, ES-shaped SearchResponse via elastic.DecodeSearchResult
//   - create: orm.Create with WaitForRefresh + WriteCreatedOKJSON
//   - update: partial-field delta with UpdatePartialFields + WriteUpdatedOKJSON
//   - get/delete: WriteGetOKJSON / WriteOpRecordNotFoundJSON envelopes
//
// Deliberately NOT imported here: core/security (it depends on core/api, so
// importing it would cycle). Callers initialize permission keys themselves
// and hand them over as strings via Config.Permission.
//
// Behavior contract is locked by crud_test.go; modules migrate onto it with
// a mechanical diff (easysearch is the pilot).
package crud

import (
	"fmt"
	"net/http"
	"strings"

	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
)

// HandlerFunc matches api.HandleUIMethod's handler signature.
type HandlerFunc func(w http.ResponseWriter, req *http.Request, ps httprouter.Params)

// Action names passed to Config.Permission.
const (
	ActionRead   = "read"
	ActionCreate = "create"
	ActionUpdate = "update"
	ActionDelete = "delete"
	ActionSearch = "search"
)

// Config parameterizes the generated CRUD. Prefix must start with "/" and
// must not end with "/" (e.g. "/easysearch"); Resource names the object for
// docs and error messages (e.g. "cluster").
// PT is the pointer type of T constrained to orm.Object — GetID/SetID have
// pointer receivers on ORMObjectBase, so the value type alone cannot satisfy
// orm.Object; this pair pattern keeps compile-time safety.
type PT[T any] interface {
	orm.Object
	*T
}

type Config[T any] struct {
	Prefix   string
	Resource string

	// Permission maps an action (ActionRead/...) to the permission key that
	// gates it. Callers create/initialize the keys with core/security
	// themselves and return api.PermissionKey strings (variety is
	// intentional: simple vs generic scopes). A nil Permission — or an
	// empty key for an action — registers that route without a permission
	// option (internal/testing use).
	Permission func(action string) api.PermissionKey

	// DefaultQueryFields backs the search endpoint's full-text/filter
	// fallback fields (e.g. []string{"name"}).
	DefaultQueryFields []string

	// SharingResource, when set, marks the ORM context with
	// orm.SharingEnabled + orm.SharingResourceType (coco's shared-resource
	// modules).
	SharingResource string

	// PrepareCreate applies defaults and validation before orm.Create
	// (ID generation, required fields, source flags...).
	PrepareCreate func(obj *T) error

	// GuardDelete vetoes deletion of protected records (e.g. reserved
	// clusters); the error message surfaces as a 403.
	GuardDelete func(obj *T) error

	// MCP exposes the five endpoints as MCP tools, named
	// <resource>_<action> (cluster_create, cluster_search, ...) — the same
	// convention coco's modules register by hand. Tool visibility and
	// invocation still pass through the route's permission option.
	MCP bool

	// MCPDescs optionally overrides the per-action tool description
	// (keyed by ActionCreate/ActionSearch/...); defaults are generated
	// from Resource.
	MCPDescs map[string]string
}

// mcpToolName/Desc resolve the MCP labels for an action.
func (cfg Config[T]) mcpToolName(action string) string {
	name := action
	if action == ActionRead {
		name = "get" // tool convention: resource_get
	}
	return fmt.Sprintf("%s_%s", cfg.Resource, name)
}

func (cfg Config[T]) mcpToolDesc(action string) string {
	if d, ok := cfg.MCPDescs[action]; ok && d != "" {
		return d
	}
	noun := cfg.Resource
	switch action {
	case ActionCreate:
		return fmt.Sprintf("Create a new %s", noun)
	case ActionSearch:
		return fmt.Sprintf("Search %ss (pagination, filters, full-text)", noun)
	case ActionRead:
		return fmt.Sprintf("Get a %s by ID", noun)
	case ActionUpdate:
		return fmt.Sprintf("Update a %s (partial fields)", noun)
	case ActionDelete:
		return fmt.Sprintf("Delete a %s by ID", noun)
	}
	return fmt.Sprintf("Call %s %s", action, noun)
}

// Handlers are the five generated endpoint functions, exposed for direct
// testing without the global router.
type Handlers struct {
	Create HandlerFunc
	Search HandlerFunc
	Get    HandlerFunc
	Update HandlerFunc
	Delete HandlerFunc
}

// generator carries the resolved config plus the embedded api.Handler for
// DecodeJSON/Write* helpers.
type generator[T any, P PT[T]] struct {
	api.Handler
	cfg Config[T]
}

// NewHandlers builds the five endpoint handlers for the config, exposed for
// direct testing without the global router.
func NewHandlers[T any, P PT[T]](cfg Config[T]) Handlers {
	g := &generator[T, P]{cfg: cfg}
	return Handlers{
		Create: g.create,
		Search: g.search,
		Get:    g.get,
		Update: g.update,
		Delete: g.delete,
	}
}

func (g *generator[T, P]) baseCtx(req *http.Request) *orm.Context {
	ctx := orm.NewContextWithParent(req.Context())
	if g.cfg.SharingResource != "" {
		ctx.Set(orm.SharingEnabled, true)
		ctx.Set(orm.SharingResourceType, g.cfg.SharingResource)
	}
	return ctx
}

func (g *generator[T, P]) create(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	obj := P(new(T))
	if err := g.DecodeJSON(req, obj); err != nil {
		g.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}
	if g.cfg.PrepareCreate != nil {
		if err := g.cfg.PrepareCreate(obj); err != nil {
			g.WriteError(w, err.Error(), http.StatusBadRequest)
			return
		}
	}
	ctx := g.baseCtx(req)
	ctx.Refresh = orm.WaitForRefresh
	if err := orm.Create(ctx, obj); err != nil {
		g.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	g.WriteCreatedOKJSON(w, obj.GetID())
}

func (g *generator[T, P]) search(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	builder, err := orm.NewQueryBuilderFromRequest(req, g.cfg.DefaultQueryFields...)
	if err != nil {
		g.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	builder.EnableBodyBytes()
	if len(builder.Sorts()) == 0 {
		builder.SortBy(orm.Sort{Field: "created", SortType: orm.DESC})
	}

	ctx := g.baseCtx(req)
	var model T
	orm.WithModel(ctx, &model)
	res, err := orm.SearchV2(ctx, builder)
	if err != nil {
		g.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	searchRes, err := elastic.DecodeSearchResult(res)
	if err != nil {
		g.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	g.WriteJSON(w, *searchRes, http.StatusOK)
}

func (g *generator[T, P]) get(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")
	obj := P(new(T))
	obj.SetID(id)
	ctx := g.baseCtx(req)
	exists, err := orm.GetV2(ctx, obj)
	if !exists || err != nil {
		g.WriteGetMissingJSON(w, id)
		return
	}
	g.WriteGetOKJSON(w, id, *obj)
}

func (g *generator[T, P]) update(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")
	obj := P(new(T))
	obj.SetID(id)

	delta := util.MapStr{}
	if err := g.DecodeJSON(req, &delta); err != nil {
		g.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx := g.baseCtx(req)
	ctx.Refresh = orm.WaitForRefresh
	if err := orm.UpdatePartialFields(ctx, obj, delta); err != nil {
		if strings.Contains(err.Error(), "not found") {
			g.WriteOpRecordNotFoundJSON(w, id)
			return
		}
		g.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	g.WriteUpdatedOKJSON(w, obj.GetID())
}

func (g *generator[T, P]) delete(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.ByName("id")
	obj := P(new(T))
	obj.SetID(id)
	ctx := g.baseCtx(req)
	exists, err := orm.GetV2(ctx, obj)
	if !exists || err != nil {
		g.WriteOpRecordNotFoundJSON(w, id)
		return
	}
	if g.cfg.GuardDelete != nil {
		if err := g.cfg.GuardDelete(obj); err != nil {
			g.WriteError(w, err.Error(), http.StatusForbidden)
			return
		}
	}
	ctx.Refresh = orm.WaitForRefresh
	if err := orm.Delete(ctx, obj); err != nil {
		g.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	g.WriteDeletedOKJSON(w, id)
}
