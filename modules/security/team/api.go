/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package team

import (
	"net/http"
	"strings"

	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/security"
	"infini.sh/framework/core/util"
)

type APIHandler struct {
	api.Handler
}

func init() {
	orm.MustRegisterSchemaWithIndexName(&Team{}, "security-team")

	readPermission := security.GetSimplePermission("generic", "security:team", security.Read)
	createPermission := security.GetSimplePermission("generic", "security:team", security.Create)
	updatePermission := security.GetSimplePermission("generic", "security:team", security.Update)
	deletePermission := security.GetSimplePermission("generic", "security:team", security.Delete)
	security.RegisterPermissionsToRole(security.RoleAdmin, readPermission, createPermission, updatePermission, deletePermission)

	handler := APIHandler{}

	api.HandleUIMethod(api.GET, "/security/teams", handler.list, api.RequirePermission(readPermission))
	api.HandleUIMethod(api.GET, "/security/teams/", handler.list, api.RequirePermission(readPermission))
	api.HandleUIMethod(api.POST, "/security/teams/", handler.create, api.RequirePermission(createPermission), api.Feature(api.FeatureCSRF))
	api.HandleUIMethod(api.GET, "/security/teams/:id", handler.get, api.RequirePermission(readPermission))
	api.HandleUIMethod(api.PUT, "/security/teams/:id", handler.update, api.RequirePermission(updatePermission), api.Feature(api.FeatureCSRF))
	api.HandleUIMethod(api.DELETE, "/security/teams/:id", handler.delete, api.RequirePermission(deletePermission), api.Feature(api.FeatureCSRF))
}

// GET /security/teams — all teams (small counts, no server-side paging).
func (h *APIHandler) list(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	ctx := orm.NewContextWithParent(req.Context())
	ctx.DirectReadAccess()
	ctx.PermissionScope(security.PermissionScopePlatform)

	teams, err := List(ctx)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusInternalServerError)
		return
	}
	h.WriteJSON(w, util.MapStr{"count": len(teams), "result": teams}, http.StatusOK)
}

// GET /security/teams/:id
func (h *APIHandler) get(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	team := Team{}
	team.ID = id
	ctx := orm.NewContextWithParent(req.Context())
	exists, err := orm.GetV2(ctx, &team)
	if err != nil {
		panic(err)
	}
	if !exists {
		api.NotFoundResponse(id)
		return
	}
	api.WriteGetOKJSON(w, id, team)
}

// POST /security/teams/ {name, description, members}
func (h *APIHandler) create(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	team := Team{}
	if err := api.DecodeJSON(req, &team); err != nil {
		panic(err)
	}
	team.Name = strings.TrimSpace(team.Name)
	if team.Name == "" {
		h.WriteError(w, "name is required", http.StatusBadRequest)
		return
	}
	team.Members = normalizeMembers(team.Members)

	ctx := orm.NewContextWithParent(req.Context())
	if err := orm.Create(ctx, &team); err != nil {
		panic(err)
	}
	h.WriteJSON(w, util.MapStr{"result": team}, http.StatusCreated)
}

// PUT /security/teams/:id {name, description, members}
func (h *APIHandler) update(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")

	incoming := Team{}
	if err := api.DecodeJSON(req, &incoming); err != nil {
		panic(err)
	}

	ctx := orm.NewContextWithParent(req.Context())
	stored := Team{}
	stored.ID = id
	exists, err := orm.GetV2(ctx, &stored)
	if err != nil {
		panic(err)
	}
	if !exists {
		api.NotFoundResponse(id)
		return
	}

	if name := strings.TrimSpace(incoming.Name); name != "" {
		stored.Name = name
	}
	stored.Description = incoming.Description
	stored.Members = normalizeMembers(incoming.Members)

	if err := orm.Update(ctx, &stored); err != nil {
		panic(err)
	}
	h.WriteJSON(w, util.MapStr{"result": stored}, http.StatusOK)
}

// DELETE /security/teams/:id
func (h *APIHandler) delete(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	team := Team{}
	team.ID = id
	ctx := orm.NewContextWithParent(req.Context())
	if err := orm.Delete(ctx, &team); err != nil {
		panic(err)
	}
	h.WriteJSON(w, util.MapStr{"ok": true}, http.StatusOK)
}

// normalizeMembers trims, drops empties and dedupes the member list.
func normalizeMembers(members []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(members))
	for _, m := range members {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		if _, ok := seen[m]; ok {
			continue
		}
		seen[m] = struct{}{}
		out = append(out, m)
	}
	return out
}
