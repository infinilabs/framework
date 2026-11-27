/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package team

import (
	"fmt"
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
	// the route already enforced generic#security:team read; bypass the
	// ORM owner check so teams created by another user (typically an
	// admin) stay readable by anyone granted the permission
	ctx.DirectReadAccess()
	exists, err := orm.GetV2(ctx, &team)
	if err != nil && !isNotFound(err) {
		panic(err)
	}
	if !exists {
		api.WriteGetMissingJSON(w, id)
		return
	}
	api.WriteGetOKJSON(w, id, team)
}

// POST /security/teams/ {name, description, members}
func (h *APIHandler) create(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	team := Team{}
	if err := api.DecodeJSON(req, &team); err != nil {
		h.WriteError(w, "invalid team input", http.StatusBadRequest)
		return
	}
	team.Name = strings.TrimSpace(team.Name)
	if team.Name == "" {
		h.WriteError(w, "name is required", http.StatusBadRequest)
		return
	}
	members, err := normalizeMembers(team.Members)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}
	team.Members = members

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
		h.WriteError(w, "invalid team input", http.StatusBadRequest)
		return
	}
	members, err := normalizeMembers(incoming.Members)
	if err != nil {
		h.WriteError(w, err.Error(), http.StatusBadRequest)
		return
	}

	ctx := orm.NewContextWithParent(req.Context())
	// The route authorizes team updates independently of object ownership.
	//
	// Bypass both ORM owner checks so authorized nonowners can update the team.
	ctx.DirectAccess()
	stored := Team{}
	stored.ID = id
	exists, err := orm.GetV2(ctx, &stored)
	if err != nil && !isNotFound(err) {
		panic(err)
	}
	if !exists {
		api.WriteGetMissingJSON(w, id)
		return
	}

	if name := strings.TrimSpace(incoming.Name); name != "" {
		stored.Name = name
	}
	stored.Description = incoming.Description
	stored.Members = members

	if err := orm.Update(ctx, &stored); err != nil {
		panic(err)
	}
	h.WriteJSON(w, util.MapStr{"result": stored}, http.StatusOK)
}

// DELETE /security/teams/:id
//
// Note on consistency: the ORM backend may index writes with a slight
// delay (Elasticsearch refresh interval), so a membership change is
// visible to logins issued immediately afterwards only eventually.
func (h *APIHandler) delete(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	id := ps.MustGetParameter("id")
	team := Team{}
	team.ID = id
	ctx := orm.NewContextWithParent(req.Context())
	// the delete path runs the ORM existence check (a read) and the
	// owner-check hook; bypass both — the route permission gates the
	// operation
	ctx.DirectAccess()
	if err := orm.Delete(ctx, &team); err != nil {
		if isNotFound(err) {
			h.WriteError(w, "team not found", http.StatusNotFound)
			return
		}
		panic(err)
	}
	h.WriteJSON(w, util.MapStr{"ok": true}, http.StatusOK)
}

// isNotFound matches the orm layer's string-typed missing-record errors
// (orm reports them as plain errors, e.g. "failed to delete, object was
// not found" and the adapters' "record not found").
func isNotFound(err error) bool {
	return err != nil && strings.Contains(err.Error(), "not found")
}

// Helper function to validate and deduplicate members for team creation and updates.
func normalizeMembers(members []MemberId) ([]MemberId, error) {
	seen := map[MemberId]struct{}{}
	out := make([]MemberId, 0, len(members))
	for _, member := range members {
		member.Value = strings.TrimSpace(member.Value)
		if member.Value == "" || (member.Kind != MemberKindUserID && member.Kind != MemberKindLogin) {
			return nil, fmt.Errorf("members require kind user_id or login and a nonempty value")
		}
		if _, duplicate := seen[member]; !duplicate {
			seen[member] = struct{}{}
			out = append(out, member)
		}
	}
	return out, nil
}
