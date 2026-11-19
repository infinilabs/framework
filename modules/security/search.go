/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package security

import (
	"net/http"

	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/security"
	"infini.sh/framework/modules/security/http_filters"
	"infini.sh/framework/modules/security/native"
	staticauth "infini.sh/framework/modules/security/static"
)

// registerSearchAPI registers the unified listing endpoints on top of the
// per-backend registrations: one route per resource, and the source query
// parameter selects where the listing comes from (staticauth.SourceStatic
// for the yml-configured registry, the ORM store otherwise). Must run
// after native.Init, which owns the registrations its backends provide.
func (module *Module) registerSearchAPI() {
	userSearch := api.RequirePermission(security.GetOrInitPermission("generic", "security:user", security.Search))
	roleSearch := api.RequirePermission(security.GetOrInitPermission("generic", "security:role", security.Search))

	// The native module only registers POST /security/user/_search; this
	// GET listing is source-dispatched so clients have one uniform call
	// per resource across deployments.
	api.HandleUIMethod(api.GET, "/security/user/_search", module.searchUsers, userSearch, api.Feature(http_filters.FeatureMaskSensitiveField))

	if !module.cfg.Authentication.Native.Enabled {
		// native.Init did not register its role search route; provide the
		// listing endpoint so roles remain inspectable in static-only
		// deployments. When native is enabled, native.SearchRole owns the
		// route and performs the same source dispatch.
		api.HandleUIMethod(api.GET, "/security/role/_search", module.searchRoles, roleSearch)
	}
}

// searchUsers serves GET /security/user/_search. source=static lists the
// yml-configured accounts; anything else goes to the native ORM search
// when that backend manages users, and returns an empty listing when it
// doesn't (native disabled, or Managed mode where user management lives
// remotely).
func (module *Module) searchUsers(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	if staticauth.IsStaticSource(req) {
		staticauth.UserSearch(w, req, ps)
		return
	}
	if module.cfg.Authentication.Native.Enabled && !module.cfg.Managed {
		native.SearchUser(w, req, ps)
		return
	}
	staticauth.WriteSearchResponse(w, nil)
}

// searchRoles serves GET /security/role/_search when the native backend
// is disabled: the static listing for source=static, an empty listing
// otherwise.
func (module *Module) searchRoles(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	if staticauth.IsStaticSource(req) {
		staticauth.RoleSearch(w, req, ps)
		return
	}
	staticauth.WriteSearchResponse(w, nil)
}
