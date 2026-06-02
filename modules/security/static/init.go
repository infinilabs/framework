/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package staticauth

import (
	"context"
	"strings"
	"sync"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/security"
)

type provider struct{}

type roleDefinition struct {
	permissions []security.PermissionKey
	grantAll    bool
}

var (
	mu          sync.RWMutex
	roles       = map[string]roleDefinition{}
	roleMapping = map[string][]string{}
)

func Init(cfg config.StaticAuthorizationConfig) {
	mu.Lock()
	defer mu.Unlock()

	roles = map[string]roleDefinition{}
	roleMapping = map[string][]string{}

	for _, role := range cfg.Roles {
		if role.Name == "" {
			continue
		}

		def := roleDefinition{}
		for _, permission := range role.Permissions {
			if permission == "" {
				continue
			}
			if permission == "*" {
				def.grantAll = true
				continue
			}
			def.permissions = append(def.permissions, security.PermissionKey(permission))
		}

		roles[role.Name] = def
		if len(def.permissions) > 0 {
			security.ReplacePermissionsForRole(role.Name, def.permissions)
		} else {
			security.ReplacePermissionsForRole(role.Name, []security.PermissionKey{})
		}
	}

	for subject, assignedRoles := range cfg.RoleMapping {
		trimmed := strings.TrimSpace(subject)
		if trimmed == "" {
			continue
		}
		roleMapping[trimmed] = append([]string(nil), assignedRoles...)
	}

	security.RegisterAuthorizationProvider("static_authorization", &provider{})
}

func (p *provider) GetPermissionKeysByUserID(ctx context.Context, providerID, userID string) []security.PermissionKey {
	return permissionsForRoles(rolesForSubject(userID))
}

func (p *provider) GetPermissionKeysByRoles(ctx context.Context, assignedRoles []string) []security.PermissionKey {
	return permissionsForRoles(assignedRoles)
}

func rolesForSubject(subject string) []string {
	if subject == "" {
		return nil
	}

	mu.RLock()
	defer mu.RUnlock()

	mapped := roleMapping[subject]
	if len(mapped) == 0 {
		return nil
	}
	return append([]string(nil), mapped...)
}

func mergeMappedRoles(subjects ...string) []string {
	merged := map[string]struct{}{}
	for _, subject := range subjects {
		for _, role := range rolesForSubject(subject) {
			if role == "" {
				continue
			}
			merged[role] = struct{}{}
		}
	}

	result := make([]string, 0, len(merged))
	for role := range merged {
		result = append(result, role)
	}
	return result
}

func permissionsForRoles(assignedRoles []string) []security.PermissionKey {
	if len(assignedRoles) == 0 {
		return nil
	}

	mu.RLock()
	defer mu.RUnlock()

	granted := map[security.PermissionKey]struct{}{}
	grantAll := false
	for _, role := range assignedRoles {
		def, ok := roles[role]
		if !ok {
			continue
		}
		if def.grantAll {
			grantAll = true
		}
		for _, permission := range def.permissions {
			granted[permission] = struct{}{}
		}
	}

	if grantAll {
		return security.GetAllPermissionKeys()
	}

	result := make([]security.PermissionKey, 0, len(granted))
	for permission := range granted {
		result = append(result, permission)
	}
	return result
}
