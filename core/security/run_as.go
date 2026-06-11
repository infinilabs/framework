/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package security

import (
	"context"
)

<<<<<<< HEAD
func RunAs(ctx context.Context,provider, userID string) context.Context {
=======
func RunAs(ctx context.Context, provider, userID string) context.Context {
>>>>>>> origin/main
	claims := UserSessionInfo{}
	claims.SetUserID(userID)

	claims.Provider = provider
	claims.Login = userID

	claims.UserAssignedPermission = GetUserPermissions(&claims)
	return AddUserToContext(ctx, &claims)
}
