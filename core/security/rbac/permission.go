/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package rbac

import "sync"

var permissionsMap = map[string]interface{}{}
var permissionsLocker = sync.RWMutex{}

func RegisterPermission(typ string, permissions interface{}){
	permissionsLocker.Lock()
	defer permissionsLocker.Unlock()
	permissionsMap[typ] = permissions
}

func GetPermissions(typ string) interface{}{
	permissionsLocker.RLocker()
	defer permissionsLocker.RUnlock()
	return  permissionsMap[typ]
}