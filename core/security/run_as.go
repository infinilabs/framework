/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package security

import (
	"context"
)

func RunAs(ctx context.Context,userID string) context.Context{
	claims :=  UserSessionInfo{}
	claims.SetGetUserID(userID)

	//claims.System = accessToken.System
	claims.Provider = "run_as"
	claims.Login = userID

	claims.UserAssignedPermission = GetUserPermissions(&claims)
	return AddUserToContext(ctx, &claims)
}