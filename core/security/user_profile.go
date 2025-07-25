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
	"infini.sh/framework/core/orm"
)

type User struct {
	UserProfile
}

type UserProfile struct {
	orm.ORMObjectBase
	Name        string      `json:"name,omitempty"  elastic_mapping:"name: { type: keyword }"`
	Email       string      `json:"email,omitempty" elastic_mapping:"email: { type: keyword }"`
	Phone       string      `json:"phone,omitempty" elastic_mapping:"phone: { type: keyword }"`
	AvatarUrl   string      `json:"avatar,omitempty" elastic_mapping:"avatar: { type: keyword }"`
	Roles       []UserRole  `json:"roles,omitempty" elastic_mapping:"roles: { type: object }"`
	Preferences Preferences `json:"preferences,omitempty" elastic_mapping:"preferences: { type: object }"`
	Payload     interface{} `json:"payload,omitempty" elastic_mapping:"payload: { enabled: false }"`
	//Tags      []string `json:"tags,omitempty" elastic_mapping:"tags: { type: keyword }"`
}

type UserExternalProfile struct {
	orm.ORMObjectBase
	AuthProvider string      `json:"provider,omitempty"  elastic_mapping:"provider: { type: keyword }"`
	Login        string      `json:"login,omitempty"  elastic_mapping:"login: { type: keyword }"`
	Name         string      `json:"name,omitempty"  elastic_mapping:"name: { type: keyword }"`
	Email        string      `json:"email,omitempty" elastic_mapping:"email: { type: keyword }"`
	AvatarUrl    string      `json:"avatar,omitempty" elastic_mapping:"avatar: { type: keyword }"`
	Payload      interface{} `json:"payload,omitempty" elastic_mapping:"payload: { type: object }"`
}

type Preferences struct {
	Theme    string `json:"theme,omitempty"`
	Language string `json:"language,omitempty"`
}
