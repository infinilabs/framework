/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package security

import (
	"context"
)

func RunAs(ctx context.Context, provider, userID string) context.Context {
	claims := UserSessionInfo{}
	claims.SetUserID(userID)

	claims.Provider = provider
	claims.Login = userID

	claims.UserAssignedPermission = GetUserPermissions(&claims)
	return AddUserToContext(ctx, &claims)
}
