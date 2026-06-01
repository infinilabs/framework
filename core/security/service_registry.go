/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package security

import (
	"context"
	"errors"
	"sync"
)

type AuthenticationBackend interface {
	GetUserByID(id string) (bool, *UserAccount, error)
	GetUserByLogin(login string) (bool, *UserAccount, error)
	CreateUser(name, login, password string, force bool) (*UserAccount, error)
}

type AuthorizationBackend interface {
	GetPermissionKeysByUserID(ctx context.Context, providerID, userID string) []PermissionKey
	GetPermissionKeysByRoles(ctx context.Context, roles []string) []PermissionKey
}

// AccountPasswordLoginProvider lets applications keep their own password-auth realms
// while reusing the shared framework account login handler and session issuance.
type AccountPasswordLoginProvider interface {
	AuthenticateByPassword(login, password string) (*UserSessionInfo, error)
}

var authorizationBackendProviders = sync.Map{}

func RegisterAuthorizationProvider(name string, provider AuthorizationBackend) {
	authorizationBackendProviders.Store(name, provider)
}

var authenticationBackendBackendProviders = sync.Map{}

func RegisterAuthenticationProvider(name string, provider AuthenticationBackend) {
	authenticationBackendBackendProviders.Store(name, provider)
}

var accountPasswordLoginProviders = sync.Map{}

func RegisterAccountPasswordLoginProvider(name string, provider AccountPasswordLoginProvider) {
	accountPasswordLoginProviders.Store(name, provider)
}

func MustGetAuthenticationProvider(provider string) AuthenticationBackend {
	value, ok := authenticationBackendBackendProviders.Load(provider)
	if ok {
		p, ok := value.(AuthenticationBackend)
		if ok {
			return p
		}
	}
	panic("AuthenticationBackend was not found")
}

func GetUserByID(id string) (string, *UserAccount, error) {
	hit := false
	var provider string
	var out *UserAccount
	authenticationBackendBackendProviders.Range(func(key, value any) bool {
		p, ok := value.(AuthenticationBackend)
		if ok {
			hit = true
			ok, v, _ := p.GetUserByID(id)
			if ok && v != nil {
				out = v
				provider = key.(string)
				return false
			}
		}
		return true
	})

	if out != nil {
		return provider, out, nil
	}

	if !hit {
		return provider, nil, errors.New("no AuthenticationBackend was found")
	}

	return provider, nil, errors.New("not found")
}

func GetUserByLogin(login string) (bool, *UserAccount, error) {
	hit := false
	var out *UserAccount
	authenticationBackendBackendProviders.Range(func(key, value any) bool {
		p, ok := value.(AuthenticationBackend)
		if ok {
			hit = true
			ok, v, _ := p.GetUserByLogin(login)
			if ok && v != nil {
				out = v
				return false
			}
		}
		return true
	})

	if out != nil {
		return true, out, nil
	}

	if !hit {
		return false, nil, errors.New("no AuthenticationBackend was found")
	}

	return false, nil, nil
}

// AuthenticateAccountPasswordLogin tries application-provided password login providers
// after the native framework account path has either not matched or not succeeded.
func AuthenticateAccountPasswordLogin(login, password string) (*UserSessionInfo, error) {
	var out *UserSessionInfo
	var lastErr error

	accountPasswordLoginProviders.Range(func(key, value any) bool {
		provider, ok := value.(AccountPasswordLoginProvider)
		if !ok {
			return true
		}

		sessionUser, err := provider.AuthenticateByPassword(login, password)
		if err != nil {
			lastErr = err
			return true
		}
		if sessionUser != nil {
			out = sessionUser
			return false
		}
		return true
	})

	if out != nil {
		return out, nil
	}
	return nil, lastErr
}
