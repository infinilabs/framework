/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package rbac

import (
	"errors"
	"fmt"
	"github.com/golang-jwt/jwt"
	"infini.sh/framework/core/api/rbac/enum"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/util"
	"net/http"
	"strings"
	"time"
)

type EsRequest struct {
	Doc       string `json:"doc"`
	Privilege string `json:"privilege"`
	ClusterRequest
	IndexRequest
}

type ClusterRequest struct {
	Cluster   []string `json:"cluster"`
	Privilege []string `json:"privilege"`
}

type IndexRequest struct {
	Cluster   []string `json:"cluster"`
	Index     []string `json:"index"`
	Privilege []string `json:"privilege"`
}

type RolePermission struct {
	Platform         []string `json:"platform,omitempty"`
	Cluster          []string `json:"cluster"`
	ClusterPrivilege []string `json:"cluster_privilege"`

	IndexPrivilege map[string][]string `json:"index_privilege"`
}

func NewIndexRequest(ps httprouter.Params, privilege []string) IndexRequest {
	index := ps.ByName("index")
	clusterId := ps.ByName("id")
	return IndexRequest{
		Cluster:   []string{clusterId},
		Index:     []string{index},
		Privilege: privilege,
	}
}

func NewClusterRequest(ps httprouter.Params, privilege []string) ClusterRequest {
	clusterId := ps.ByName("id")
	return ClusterRequest{
		Cluster:   []string{clusterId},
		Privilege: privilege,
	}
}

func ValidateIndex(req IndexRequest, userRole RolePermission) (err error) {

	userClusterMap := make(map[string]struct{})
	for _, v := range userRole.Cluster {
		userClusterMap[v] = struct{}{}
	}
	for _, v := range req.Cluster {
		if _, ok := userClusterMap[v]; !ok {
			err = errors.New("no cluster permission")
			return
		}
	}

	for _, val := range req.Privilege {
		position := strings.Index(val, ".")
		if position == -1 {
			err = errors.New("invalid privilege parameter")
			return err
		}
		prefix := val[:position]
		for _, v := range req.Index {
			privilege, ok := userRole.IndexPrivilege[v]
			if !ok {
				err = errors.New("no index permission")
				return err
			}
			if util.StringInArray(privilege, prefix+".*") {
				continue
			}
			if util.StringInArray(privilege, val) {
				continue
			}
			return fmt.Errorf("no index api permission: %s", val)
		}
	}

	return nil
}

func ValidateCluster(req ClusterRequest, roleNames []string) (err error) {
	userClusterMap := GetRoleClusterMap(roleNames)
	for _, v := range req.Cluster {
		userClusterPermissions, ok := userClusterMap[v]
		if !ok && userClusterMap["*"] == nil{
			err = fmt.Errorf("no cluster[%s] permission", v)
			return
		}
		if util.StringInArray(userClusterPermissions, "*") {
			continue
		}
		// if include api.*  for example: cat.* , return nil
		for _, privilege := range req.Privilege {
			prefix := privilege[:strings.Index(privilege, ".")]

			if util.StringInArray(userClusterPermissions, prefix+".*") {
				continue
			}
			if util.StringInArray(userClusterPermissions, privilege) {
				continue
			}
			return fmt.Errorf("no cluster api permission: %s", privilege)
		}
	}
	return nil

}

func CombineUserRoles(roleNames []string) RolePermission {
	newRole := RolePermission{}
	m := make(map[string][]string)
	for _, val := range roleNames {
		role := RoleMap[val]
		for _, v := range role.Privilege.Elasticsearch.Cluster.Resources {
			newRole.Cluster = append(newRole.Cluster, v.ID)
		}
		for _, v := range role.Privilege.Elasticsearch.Cluster.Permissions {
			newRole.ClusterPrivilege = append(newRole.ClusterPrivilege, v)
		}
		for _, v := range role.Privilege.Platform {
			newRole.Platform = append(newRole.Platform, v)
		}
		for _, v := range role.Privilege.Elasticsearch.Index {

			for _, name := range v.Name {
				if _, ok := m[name]; ok {
					m[name] = append(m[name], v.Permissions...)
				} else {
					m[name] = v.Permissions
				}

			}

		}
	}
	newRole.IndexPrivilege = m
	return newRole
}

func GetRoleClusterMap(roles []string) map[string][]string {
	userClusterMap := make(map[string][]string, 0)
	for _, roleName := range roles {
		role, ok := RoleMap[roleName]
		if ok {
			for _, ic := range role.Privilege.Elasticsearch.Cluster.Resources {
				userClusterMap[ic.ID] = append(userClusterMap[ic.ID], role.Privilege.Elasticsearch.Cluster.Permissions...)
			}
		}
	}
	return userClusterMap
}
//GetRoleCluster get cluster id by given role names
//return true when has all cluster privilege, otherwise return cluster id list
func GetRoleCluster(roles []string) (bool, []string) {
	userClusterMap := GetRoleClusterMap(roles)
	if _, ok := userClusterMap["*"]; ok {
		return true, nil
	}
	realCluster := make([]string, 0, len(userClusterMap))
	for k, _ := range userClusterMap {
		realCluster = append(realCluster, k)
	}
	return false, realCluster
}

//GetCurrentUserCluster get cluster id by current login user
//return true when has all cluster privilege, otherwise return cluster id list
func GetCurrentUserCluster(req *http.Request) (bool, []string){
	ctxVal := req.Context().Value("user")
	if userClaims, ok := ctxVal.(*UserClaims); ok {
		return GetRoleCluster(userClaims.Roles)
	}else{
		panic("user context value not found")
	}
}

func GetRoleIndex(roles []string, clusterID string) (bool, []string){
	var realIndex []string
	for _, roleName := range roles {
		role, ok := RoleMap[roleName]
		if ok {
			for _, ic := range role.Privilege.Elasticsearch.Cluster.Resources {
				if ic.ID != "*" && ic.ID != clusterID {
					continue
				}
				for _, ip := range role.Privilege.Elasticsearch.Index {
					if util.StringInArray(ip.Name, "*"){
						return true, nil
					}
					realIndex = append(realIndex, ip.Name...)
				}
			}
		}
	}
	return false, realIndex
}

func GetCurrentUserIndex(req *http.Request, clusterID string) (bool, []string){
	ctxVal := req.Context().Value("user")
	if userClaims, ok := ctxVal.(*UserClaims); ok {
		return GetRoleIndex(userClaims.Roles, clusterID)
	}else{
		panic("user context value not found")
	}
}

func ValidateLogin(authorizationHeader string) (clams *UserClaims, err error) {

	if authorizationHeader == "" {
		err = errors.New("authorization header is empty")
		return
	}
	fields := strings.Fields(authorizationHeader)
	if fields[0] != "Bearer" || len(fields) != 2 {
		err = errors.New("authorization header is invalid")
		return
	}
	tokenString := fields[1]

	token, err := jwt.ParseWithClaims(tokenString, &UserClaims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(Secret), nil
	})
	if err != nil {
		return
	}
	clams, ok := token.Claims.(*UserClaims)

	if clams.UserId == "" {
		err = errors.New("user id is empty")
		return
	}
	//fmt.Println("user token", clams.UserId, TokenMap[clams.UserId])
	tokenVal := GetUserToken(clams.UserId)
	if tokenVal == nil {
		err = errors.New("token is invalid")
		return
	}
	if tokenVal.ExpireIn < time.Now().Unix() {
		err = errors.New("token is expire in")
		DeleteUserToken(clams.UserId)
		return
	}
	if ok && token.Valid {
		return clams, nil
	}
	return

}

func ValidatePermission(claims *UserClaims, permissions []string) (err error) {

	user := claims.ShortUser

	if user.UserId == "" {
		err = errors.New("user id is empty")
		return
	}
	if user.Roles == nil {
		err = errors.New("api permission is empty")
		return
	}

	// 权限校验
	userPermissions := make([]string, 0)
	for _, role := range user.Roles {
		if _, ok := RoleMap[role]; ok {
			for _, v := range RoleMap[role].Privilege.Platform {
				userPermissions = append(userPermissions, v)
			}
		}
	}
	userPermissionMap := make(map[string]struct{})
	for _, val := range userPermissions {
		for _, v := range enum.PermissionMap[val] {
			userPermissionMap[v] = struct{}{}
		}

	}

	for _, v := range permissions {
		if _, ok := userPermissionMap[v]; !ok {
			err = errors.New("permission denied")
			return
		}
	}
	return nil

}

