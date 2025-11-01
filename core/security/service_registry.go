/* Copyright © INFINI LTD. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package security

import (
	"errors"
	"infini.sh/framework/core/util"
	"sync"
)

type AuthenticationBackend interface {
	//Auth(provider, login string)
	GetUserByID(id string) (bool, *UserAccount, error)
	GetUserByLogin(login string) (bool, *UserAccount, error)
	CreateUser(name, login, password string, force bool) (*UserAccount, error)
}

type AuthorizationBackend interface {
	GetPermissionKeysByUserID(userID string) []PermissionKey
	GetPermissionKeysByRoles(roles []string) []PermissionKey
}

var authorizationBackendProviders=sync.Map{}

func RegisterAuthorizationProvider(name string,provider AuthorizationBackend)  {
	authorizationBackendProviders.Store(name,provider)
}

var authenticationBackendBackendProviders=sync.Map{}

func RegisterAuthenticationProvider(name string,provider AuthenticationBackend)  {
	authenticationBackendBackendProviders.Store(name,provider)
}

func MustGetAuthenticationProvider() AuthenticationBackend {
	var provider AuthenticationBackend
	authorizationBackendProviders.Range(func(key, value any) bool {
		p,ok:=value.(AuthenticationBackend)
		if ok{
			provider=p
			return false
			}

		return true
	})
	if provider!=nil{
		return provider
	}

	panic("no AuthenticationBackend was found")
}

func GetUserByID(id string) (bool, *UserAccount, error) {
	hit:=false
	var out *UserAccount
	authorizationBackendProviders.Range(func(key, value any) bool {
		p,ok:=value.(AuthenticationBackend)
		if ok{
			hit=true
			ok,v,_:=p.GetUserByID(id)
			if ok&&v!=nil{
				out=v
				return false
			}
		}
		return true
	})

	if out!=nil{
		return true,out,nil
	}

	if !hit{
		panic("no AuthenticationBackend was found")
	}

	return false,nil,errors.New("not found")
}

func GetUserByLogin(login string) (bool, *UserAccount, error) {
	hit:=false
	var out *UserAccount
	authorizationBackendProviders.Range(func(key, value any) bool {
		p,ok:=value.(AuthenticationBackend)
		if ok{
			hit=true
			ok,v,_:=p.GetUserByLogin(login)
			if ok&&v!=nil{
				out=v
				return false
			}
		}
		return true
	})

	if out!=nil{
		return true,out,nil
	}

	if !hit{
		panic("no AuthenticationBackend was found")
	}

	return false,nil,errors.New("not found")
}

func GetPermissionKeysByUserID(userID string)[]PermissionKey  {
	hit:=false
	out:=[]PermissionKey{}
	authorizationBackendProviders.Range(func(key, value any) bool {
		p,ok:=value.(AuthorizationBackend)
		if ok{
			hit=true
			v:=p.GetPermissionKeysByUserID(userID)
			out=append(out,v...)
		}
		return true
	})

	if !hit{
		panic("no AuthorizationBackend was found")
	}

	return out
}

func GetPermissionKeysByRole(roles []string) []PermissionKey {

	//for admin only
	if util.ContainsAnyInArray(RoleAdmin, roles) {
		permissions := GetAllPermissionKeys()
		return permissions
	}

	var hit=false
	permissions := []PermissionKey{}
	authorizationBackendProviders.Range(func(key, value any) bool {
		p,ok:=value.(AuthorizationBackend)
		if ok{
			hit=true
			v:=p.GetPermissionKeysByRoles(roles)
			permissions=append(permissions,v...)
		}
		return true
	})

	if !hit{
		panic("no AuthorizationBackend was found")
	}

	return permissions
}