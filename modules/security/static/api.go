/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package staticauth

import (
	"net/http"

	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/util"
)

// SourceStatic is the source query-parameter value that routes a
// /security/user/_search or /security/role/_search request to the static
// (yml-configured) registry instead of the ORM store.
const SourceStatic = "static"

// IsStaticSource reports whether the request asks for the static source.
func IsStaticSource(req *http.Request) bool {
	return req.URL.Query().Get("source") == SourceStatic
}

// UserSearch serves /security/user/_search?source=static: a read-only echo
// of the yml static accounts, in the same envelope and item shape as the
// ORM-backed search (hits/_id/_source), so clients can render both
// listings with one model. Full list, no pagination: the source is the
// config file, so the result is bounded by it. created/updated stay
// absent — config-defined accounts have no ORM metadata — and each item
// carries source: "static". ListAccounts returns sanitized copies without
// the bcrypt password hashes.
func UserSearch(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	accounts := ListAccounts()
	docs := make([]elastic.DocumentWithMeta[util.MapStr], 0, len(accounts))
	for _, a := range accounts {
		docs = append(docs, elastic.DocumentWithMeta[util.MapStr]{
			ID: a.ID,
			Source: util.MapStr{
				"id": a.ID, "name": a.Name, "email": a.Email,
				"roles": a.Roles, "source": SourceStatic,
			},
		})
	}
	WriteSearchResponse(w, docs)
}

// RoleSearch serves /security/role/_search?source=static: a read-only
// echo of the yml static roles (authorization.static.roles), full list
// for the same reason as UserSearch. id repeats the name:
// config-defined roles are keyed by name, and the field gives clients a
// uniform row key across sources. permissions nest under grants like the
// ORM-backed role items; the wildcard entry is consumed into grantAll at
// load time, so grantAll roles report grant_all: true with an empty
// permissions list, not permissions: ["*"].
func RoleSearch(w http.ResponseWriter, req *http.Request, _ httprouter.Params) {
	defs := ListRoleDefinitions()
	docs := make([]elastic.DocumentWithMeta[util.MapStr], 0, len(defs))
	for _, d := range defs {
		perms := make([]string, 0, len(d.Permissions))
		for _, p := range d.Permissions {
			perms = append(perms, string(p))
		}
		docs = append(docs, elastic.DocumentWithMeta[util.MapStr]{
			ID: d.Name,
			Source: util.MapStr{
				"id": d.Name, "name": d.Name,
				"grants":    util.MapStr{"permissions": perms},
				"grant_all": d.GrantAll,
				"source":    SourceStatic,
			},
		})
	}
	WriteSearchResponse(w, docs)
}

// WriteSearchResponse emits docs in the SearchResponseWithMeta envelope
// used by the ORM-backed _search handlers; nil yields an empty listing.
// Shared with the security module, which serves the same envelope for
// sources that have nothing to list.
func WriteSearchResponse(w http.ResponseWriter, docs []elastic.DocumentWithMeta[util.MapStr]) {
	res := elastic.SearchResponseWithMeta[util.MapStr]{}
	res.Hits.Hits = docs
	res.Hits.Total = elastic.NewGeneralTotal(int64(len(docs)))
	api.WriteJSON(w, res, http.StatusOK)
}
