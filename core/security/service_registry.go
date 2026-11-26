/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package security

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

type AuthenticationBackend interface {
	GetUserByID(id string) (bool, *UserAccount, error)
	GetUserByLogin(login string) (bool, *UserAccount, error)
	CreateUser(name, login, password string, force bool) (*UserAccount, error)
}

type AuthorizationBackend interface {
	GetPermissionKeysByUserID(ctx context.Context, providerID, userID, login string) []PermissionKey
	GetPermissionKeysByRoles(ctx context.Context, roles []string) []PermissionKey
}

var authorizationBackendProviders = sync.Map{}

func RegisterAuthorizationProvider(name string, provider AuthorizationBackend) {
	authorizationBackendProviders.Store(name, provider)
}

var authenticationBackendBackendProviders = sync.Map{}

func RegisterAuthenticationProvider(name string, provider AuthenticationBackend) {
	authenticationBackendBackendProviders.Store(name, provider)
}

// ErrUserNotFound means every authentication provider reported no matching account.
var ErrUserNotFound = errors.New("user not found")

// ErrIdentityConflict reports an ID or login already used by another account.
var ErrIdentityConflict = errors.New("account identity conflict")

// GetAuthenticationProvider returns a registered authentication backend by name.
func GetAuthenticationProvider(name string) (AuthenticationBackend, bool) {
	value, ok := authenticationBackendBackendProviders.Load(name)
	if !ok {
		return nil, false
	}
	provider, ok := value.(AuthenticationBackend)
	return provider, ok
}

func MustGetAuthenticationProvider(name string) AuthenticationBackend {
	if provider, ok := GetAuthenticationProvider(name); ok {
		return provider
	}
	panic("AuthenticationBackend was not found")
}

// GetUserByID finds an account and its authentication provider by ID.
func GetUserByID(id string) (string, *UserAccount, error) {
	return findAccount(id, true)
}

// ResolveUserByLogin finds an account and its authentication provider by login.
func ResolveUserByLogin(login string) (string, bool, *UserAccount, error) {
	provider, account, err := findAccount(login, false)
	if err != nil {
		return "", false, nil, err
	}
	return provider, true, account, nil
}

// GetUserByLogin finds an account by login.
func GetUserByLogin(login string) (bool, *UserAccount, error) {
	_, found, account, err := ResolveUserByLogin(login)
	return found, account, err
}

// Helper function to find an account by ID or login across registered providers.
func findAccount(identity string, byID bool) (string, *UserAccount, error) {
	var provider string
	var account *UserAccount
	var lookupErr error
	var found, registered bool
	authenticationBackendBackendProviders.Range(func(key, value any) bool {
		registered = true
		backend := value.(AuthenticationBackend)
		if byID {
			found, account, lookupErr = backend.GetUserByID(identity)
		} else {
			found, account, lookupErr = backend.GetUserByLogin(identity)
		}
		if lookupErr != nil {
			lookupErr = fmt.Errorf("authentication provider %q: %w", key, lookupErr)
			return false
		}
		if found {
			provider = key.(string)
			return false
		}
		return true
	})
	if !registered {
		return "", nil, errors.New("no AuthenticationBackend was found")
	}
	if lookupErr != nil {
		return "", nil, lookupErr
	}
	if !found {
		return "", nil, ErrUserNotFound
	}
	return provider, account, nil
}
