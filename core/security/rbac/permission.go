/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package rbac

import "sync"

var permissionsMap = map[string]interface{}{}
var permissionsLocker = sync.Mutex{}

func RegisterPermission(typ string, permissions interface{}){
	permissionsLocker.Lock()
	defer permissionsLocker.Unlock()
	permissionsMap[typ] = permissions
}

func GetPermissions(typ string) interface{}{
	permissionsLocker.Lock()
	defer permissionsLocker.Unlock()
	return  permissionsMap[typ]
}

var RoleMap = make(map[string]Role)

type Token struct {
	JwtStr   string `json:"jwt_str"`
	Value    string `json:"value"`
	ExpireIn int64  `json:"expire_in"`
}

var TokenMap = make(map[string]Token)