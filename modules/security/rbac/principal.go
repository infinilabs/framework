/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package rbac

import (
	"net/http"

	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/security"
)

func SearchPrincipals(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {

	builder, err := orm.NewQueryBuilderFromRequest(req, "id", "name", "email")
	if err != nil {
		panic(err)
	}
	ctx := orm.NewContextWithParent(req.Context())
	ctx.DirectReadAccess()
	orm.WithModel(ctx, &security.UserAccount{})
	out := []security.UserAccount{}
	err, res := elastic.SearchV2WithResultItemMapper(ctx, &out, builder, nil)
	if err != nil {
		panic(err)
	}

	// use the generic type correctly
	var docs []elastic.DocumentWithMeta[security.OrganizationPrincipal]
	for _, v := range out {
		x := security.OrganizationPrincipal{
			Name:        v.Name,
			ID:          v.ID,
			Type:        security.PrincipalTypeUser,
			Description: v.Email,
		}

		doc := elastic.DocumentWithMeta[security.OrganizationPrincipal]{
			ID:     v.ID,
			Source: x,
		}

		docs = append(docs, doc)
	}

	result := elastic.SearchResponseWithMeta[security.OrganizationPrincipal]{}
	result.Hits.Hits = docs
	result.Hits.Total = elastic.NewGeneralTotal(res.Total)

	api.WriteJSON(w, result, 200)
}
