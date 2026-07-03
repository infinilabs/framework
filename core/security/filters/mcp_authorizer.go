/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package filters

import (
	"net/http"

	"infini.sh/framework/core/api"
	common "infini.sh/framework/core/api/common"
	"infini.sh/framework/core/security"
	"infini.sh/framework/core/util"
)

func init() {
	api.RegisterMCPToolAuthorizer(func(headers http.Header, options *api.HandlerOptions) bool {
		if options == nil || !common.IsAuthEnable() {
			return true
		}
		if !options.RequireLogin && !options.OptionLogin && len(options.RequirePermission) == 0 {
			return true
		}

		req := &http.Request{Header: headers}
		user, err := security.ValidateLogin(nil, req)
		if err != nil || user == nil || !user.IsValid() {
			// optional-login routes can still be called anonymously unless explicit permissions are required
			return options.OptionLogin && len(options.RequirePermission) == 0
		}

		if len(options.RequirePermission) == 0 {
			return true
		}

		if user.Roles != nil && util.AnyInArrayEquals(user.Roles, security.RoleAdmin) {
			return true
		}

		if user.UserAssignedPermission == nil || user.UserAssignedPermission.NeedRefresh() {
			user.UserAssignedPermission = security.GetUserPermissions(user)
		}
		if user.UserAssignedPermission == nil {
			return false
		}
		return user.UserAssignedPermission.Validate(security.MustRegisterPermissionByKeys(options.RequirePermission))
	})
}
