// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package security

import (
	"fmt"
	"time"

	log "github.com/cihub/seelog"
	"github.com/golang-jwt/jwt/v4"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/param"
	"infini.sh/framework/core/util"
)

type UserClaims struct {
	*jwt.RegisteredClaims
	*UserSessionInfo
}

func NewUserClaims() *UserClaims {
	return &UserClaims{
		RegisteredClaims: &jwt.RegisteredClaims{},
		UserSessionInfo:  &UserSessionInfo{},
	}
}

// auth user info
type UserSessionInfo struct {
	param.Parameters
	//user identity provided by external providers
	//Source   string `json:"source"`
	Provider string `json:"provider"` //auth provider
	Login    string `json:"login"`    //auth login

	//system level security's info
	Roles       []string        `json:"roles"`

	//private fields
	UserID string `json:"userid"` //system level user ID

	//unified permissions
	*UserAssignedPermission

	//SessionExpireAt *time.Time `json:"session_expire_at,omitempty"`

	//stats
	LastLogin LastLogin `json:"last_login,omitempty"`
}

func (u *UserSessionInfo) ToString() string {
	return fmt.Sprintf("user_id: %v, provider: %v, login: %v", u.UserID, u.Provider, u.Login)
}

func (u *UserSessionInfo) SetUserID(uid string) {
	u.UserID = uid
}

func (u *UserSessionInfo) MustGetUserID() string {
	if u.UserID != "" {
		return u.UserID
	}

	panic(errors.NewWithHTTPCode(401, "invalid user"))
}

func (u *UserSessionInfo) IsValid() bool {
	v := u.Provider != "" && u.Login != "" && u.UserID != ""
	if !v {
		if global.Env().IsDebug {
			log.Error(util.MustToJSON(u), u.UserID)
		}
		return false
	}
	return v
}

func (u *UserSessionInfo) GetKey() string {
	return fmt.Sprintf("%v-%v-%v", u.Provider, u.Login, u.UserID)
}

type LastLogin struct {
	Timestamp *time.Time `json:"timestamp,omitempty"`
	ClientIP  string     `json:"client_ip,omitempty"`
}
