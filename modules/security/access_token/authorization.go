/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package access_token

import (
	"context"

	"infini.sh/framework/core/log"
	"infini.sh/framework/core/security"
)

func init() {
	provider := SecurityBackendProvider{}
	security.RegisterAuthorizationProvider(ProviderName, &provider)
}

type SecurityBackendProvider struct {
}

func (provider *SecurityBackendProvider) GetPermissionKeysByUserID(ctx1 context.Context, providerID, userID, login string) []security.PermissionKey {
	var allowedPermissions = []security.PermissionKey{}

	if providerID == ProviderName {
		_, permissions, err := getTokenPermissions(userID)

		if err != nil {
			log.Error(err)
		} else {
			return permissions
		}
	}

	return allowedPermissions
}

func (provider *SecurityBackendProvider) GetPermissionKeysByRoles(ctx context.Context, roles []string) []security.PermissionKey {
	return []security.PermissionKey{}
}
