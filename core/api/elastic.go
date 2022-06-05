package api

import (
	"infini.sh/framework/core/api/rbac"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/radix"
	"infini.sh/framework/core/util"
	"net/http"
)

func (handler Handler) IndexRequired(h httprouter.Handle, route ...string) httprouter.Handle {

	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {
		if authEnabled {
			claims, err := rbac.ValidateLogin(r.Header.Get("Authorization"))
			if err != nil {
				handler.WriteError(w, err.Error(), http.StatusUnauthorized)
				return
			}
			newRole := rbac.CombineUserRoles(claims.Roles)

			indexReq := rbac.NewIndexRequest(ps, route)

			err = rbac.ValidateIndex(indexReq, newRole)
			if err != nil {
				handler.WriteError(w, err.Error(), http.StatusForbidden)
				return
			}
		}

		h(w, r, ps)
	}
}

func (handler Handler) ClusterRequired(h httprouter.Handle, route ...string) httprouter.Handle {

	return func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {

		if authEnabled {
			claims, err := rbac.ValidateLogin(r.Header.Get("Authorization"))
			if err != nil {
				handler.WriteError(w, err.Error(), http.StatusUnauthorized)
				return
			}
			//newRole := biz.CombineUserRoles(claims.Roles)
			clusterReq := rbac.NewClusterRequest(ps, route)
			newRole := rbac.CombineUserRoles(claims.Roles)
			err = rbac.ValidateCluster(clusterReq, newRole)
			if err != nil {
				handler.WriteError(w, err.Error(), http.StatusForbidden)
				return
			}
		}

		h(w, r, ps)
	}
}

func (handler Handler) GetClusterFilter(r *http.Request, field string) (util.MapStr, bool) {
	if !IsAuthEnable(){
		return nil, true
	}
	hasAllPrivilege, clusterIds := rbac.GetCurrentUserCluster(r)
	if hasAllPrivilege {
		return nil, true
	}
	if len(clusterIds) == 0 {
		return nil, false
	}
	return util.MapStr{
		"terms": util.MapStr{
			field: clusterIds,
		},
	}, false
}
func (handler Handler) GetAllowedClusters(r *http.Request) ([]string, bool) {
	if !IsAuthEnable(){
		return nil, true
	}
	hasAllPrivilege, clusterIds := rbac.GetCurrentUserCluster(r)
	return clusterIds, hasAllPrivilege
}

func (handler Handler) GetAllowedIndices(r *http.Request, clusterID string) ([]string, bool) {
	if !IsAuthEnable(){
		return nil, true
	}
	hasAllPrivilege, indices := rbac.GetCurrentUserClusterIndex(r, clusterID)
	if hasAllPrivilege {
		return nil, true
	}
	return indices, false
}

func (handler Handler) IsIndexAllowed(r *http.Request, clusterID string, indexName string) bool {
	if !IsAuthEnable() {
		return true
	}
	hasAllPrivilege, indices := rbac.GetCurrentUserClusterIndex(r, clusterID)
	if hasAllPrivilege {
		return true
	}
	if len(indices) == 0 {
		return false
	}
	return radix.Compile(indices...).Match(indexName)
}