/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package staticauth

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"golang.org/x/crypto/bcrypt"

	"infini.sh/framework/core/config"
	"infini.sh/framework/core/security"
)

type provider struct{}

type roleDefinition struct {
	permissions []security.PermissionKey
	grantAll    bool
}

var (
	mu               sync.RWMutex
	roles            = map[string]roleDefinition{}
	roleMapping      = map[string][]string{}
	authUsersByLogin = map[string]*security.UserAccount{}
	authUsersByID    = map[string]*security.UserAccount{}
)

// InitAuthentication registers the static authentication backend and loads the
// static user registry (with per-user roles) from config. No-op unless static
// authentication is enabled, so the password-login surface is only exposed
// when the feature is actually turned on.
func InitAuthentication(authCfg config.StaticAuthenticationConfig) {
	if !authCfg.Enabled {
		return
	}
	mu.Lock()
	defer mu.Unlock()

	authUsersByLogin = map[string]*security.UserAccount{}
	authUsersByID = map[string]*security.UserAccount{}

	for _, user := range authCfg.Users {
		login := strings.TrimSpace(user.Login)
		if login == "" {
			continue
		}

		id := strings.TrimSpace(user.ID)
		if id == "" {
			id = login
		}

		account := &security.UserAccount{
			Name:  user.Name,
			Email: login,
			Roles: append([]string(nil), user.Roles...),
		}
		account.ID = id

		if user.Password != "" {
			hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
			if err != nil {
				panic(err)
			}
			account.Password = string(hash)
		}

		authUsersByLogin[login] = account
		authUsersByID[id] = account
	}

	security.RegisterAuthenticationProvider(security.StaticAuthBackend, &provider{})
}

// InitAuthorization registers the static authorization provider and loads the
// static role definitions and the login/id → role mapping from config. No-op
// unless static authorization is enabled.
//
// The provider still consults the static user registry (loaded by
// InitAuthentication) for per-user roles when a request is authenticated by
// the static backend; that registry is only populated when authentication is
// also enabled, which is the only case those per-user roles can apply.
func InitAuthorization(cfg config.StaticAuthorizationConfig) {
	if !cfg.Enabled {
		return
	}
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

func (p *provider) GetUserByLogin(login string) (bool, *security.UserAccount, error) {
	login = strings.TrimSpace(login)
	if login == "" {
		return false, nil, nil
	}

	mu.RLock()
	defer mu.RUnlock()

	account, ok := authUsersByLogin[login]
	if !ok {
		return false, nil, nil
	}

	return true, account, nil
}

func (p *provider) GetUserByID(id string) (bool, *security.UserAccount, error) {
	if id == "" {
		return false, nil, nil
	}

	mu.RLock()
	defer mu.RUnlock()

	account, ok := authUsersByID[id]
	if !ok {
		return false, nil, nil
	}

	return true, account, nil
}

func (p *provider) CreateUser(name, login, password string, force bool) (*security.UserAccount, error) {
	return nil, fmt.Errorf("static authentication provider does not support user creation")
}

func (p *provider) GetPermissionKeysByUserID(ctx context.Context, providerID, userID, login string) []security.PermissionKey {
	var keys []security.PermissionKey
	keys = append(keys, permissionsForRoles(rolesForSubject(userID))...)
	if login != "" && login != userID {
		keys = append(keys, permissionsForRoles(rolesForSubject(login))...)
	}

	if providerID == security.StaticAuthBackend {
		mu.RLock()
		defer mu.RUnlock()
		if account, ok := authUsersByID[userID]; ok {
			keys = append(keys, permissionsForRoles(account.Roles)...)
		} else if account, ok := authUsersByLogin[login]; ok {
			keys = append(keys, permissionsForRoles(account.Roles)...)
		}
	}

	return keys
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
