package middleware

import (
	httprouter "infini.sh/framework/core/api/router"
	"net/http"
)

func IndexRequired(h httprouter.Handle, route ...string) httprouter.Handle {

	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		claims, err := ValidateLogin(r.Header.Get("Authorization"))
		if err != nil {
			w = handleError(w, http.StatusUnauthorized, err)
			return
		}
		newRole := CombineUserRoles(claims.Roles)

		indexReq := NewIndexRequest(ps, route)

		err = ValidateIndex(indexReq, newRole)
		if err != nil {
			w = handleError(w, http.StatusForbidden, err)
			return
		}
		h(w, r, ps)
	}
}
func ClusterRequired(h httprouter.Handle, route ...string) httprouter.Handle {

	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		claims, err := ValidateLogin(r.Header.Get("Authorization"))
		if err != nil {
			w = handleError(w, http.StatusUnauthorized, err)
			return
		}
		//newRole := biz.CombineUserRoles(claims.Roles)
		clusterReq := NewClusterRequest(ps, route)

		err = ValidateCluster(clusterReq, claims.Roles)
		if err != nil {
			w = handleError(w, http.StatusForbidden, err)
			return
		}
		h(w, r, ps)
	}
}
