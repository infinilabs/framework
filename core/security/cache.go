/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package security

import (
	"time"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
	ccache "infini.sh/framework/lib/cache"
)

var permissionCache = ccache.Layered(ccache.Configure().MaxSize(10000).ItemsToPrune(100))

func GetUserPermissions(shortUser *UserSessionInfo) *UserAssignedPermission {

	var skipCache = false
	if shortUser.UserAssignedPermission != nil && shortUser.UserAssignedPermission.NeedRefresh() {
		skipCache = true
	}

	if !skipCache && (shortUser.UserAssignedPermission == nil || (shortUser.UserAssignedPermission != nil && !shortUser.UserAssignedPermission.NeedRefresh())) {
		v := permissionCache.Get(PermissionCache, shortUser.GetKey())
		if v != nil {
			if !v.Expired() {
				x, ok := v.Value().(*UserAssignedPermission)
				if ok {
					if !x.NeedRefresh() {
						shortUser.UserAssignedPermission = x
						if global.Env().IsDebug {
							log.Trace("hit permission cache")
							x.Dump()
						}
						return x
					} else {
						if global.Env().IsDebug {
							log.Trace("hit permission cache, but invalid, need refresh")
						}
					}
				}
			}
		}
	}

	//TODO cache, refresh user's role from db
	//TODO, handle api key, with specify permissions
	//TODO, if the provider is for user, like api token, we need to fetch from api token's config, to get the updated permission
	allowedPermissions := MustGetPermissionKeysByUser(shortUser)

	log.Trace("get user's permissions:", allowedPermissions)
	perms := NewUserAssignedPermission(allowedPermissions, nil)
	if perms != nil {
		shortUser.UserAssignedPermission = perms
		permissionCache.Set(PermissionCache, shortUser.GetKey(), perms, util.GetDurationOrDefault("30m", time.Duration(30)*time.Minute))
		return perms
	}
	return nil
}

const PermissionCache = "UserPermissionCache"
