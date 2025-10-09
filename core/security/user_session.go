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
	"github.com/golang-jwt/jwt"
	"infini.sh/framework/core/util"
	"time"
)

type UserClaims struct {
	*jwt.RegisteredClaims
	*UserSessionInfo
}

func NewUserClaims() *UserClaims {
	return &UserClaims{
		RegisteredClaims: &jwt.RegisteredClaims{},
		UserSessionInfo: &UserSessionInfo{
			System: map[string]interface{}{},
		},
	}
}

type UserSessionInfo struct {
	System map[string]interface{} `json:"_system,omitempty" elastic_mapping:"_system: { type: object }"`

	//user identity provided by external providers
	Source   string `json:"source"`
	Provider string `json:"provider"`
	Login    string `json:"login"`

	//system level security's info
	Roles []string `json:"roles,omitempty"`

	Labels util.MapStr `json:"labels,omitempty"`

	//unified permissions
	*UserAssignedPermission

	//SessionExpireAt *time.Time `json:"session_expire_at,omitempty"`

	//stats
	LastLogin LastLogin `json:"last_login,omitempty"`
}

func (u *UserSessionInfo) ValidInfo() bool {
	return u.Provider != "" && u.Login != ""
}

func (u *UserSessionInfo) GetKey() string {
	return fmt.Sprintf("%v-%v", u.Provider, u.Login)
}

type LastLogin struct {
	Timestamp *time.Time `json:"timestamp,omitempty"`
	ClientIP  string     `json:"client_ip,omitempty"`
}
