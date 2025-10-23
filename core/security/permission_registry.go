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
	"infini.sh/framework/core/errors"
	"strings"
	"sync"
)

type PermissionRegistry struct {
	locker           sync.RWMutex
	nextPermissionID PermissionID
	permMap          map[string]PermissionID // "category#resource:resource1/action" => PermissionID
	revPermMap       map[PermissionID]string // PermissionID => "category#resource:resource1/action"
}

type PermissionItem struct {
	ID          string `json:"id"`
	Description string `json:"description,omitempty"`
	Category    string `json:"category"`
	Resource    string `json:"resource"`
	Action      string `json:"action"`
	Order       string `json:"order,omitempty"`
}

func parsePermissionKey(key string) (category, resource, action string) {
	category, resource, action = "", "", ""

	mainPart := key
	if idx := strings.IndexByte(key, '#'); idx != -1 {
		category = key[:idx]
		mainPart = key[idx+1:]
	}

	if idx := strings.IndexByte(mainPart, '/'); idx != -1 {
		resource = mainPart[:idx]
		action = mainPart[idx+1:]
	} else {
		resource = mainPart
	}

	return
}

func GetPermissionItems() []PermissionItem {
	permissionRegistry.locker.RLock()
	defer permissionRegistry.locker.RUnlock()

	items := make([]PermissionItem, 0, len(permissionRegistry.revPermMap))
	for _, key := range permissionRegistry.revPermMap {
		category := ""
		resource := ""
		action := ""

		category, resource, action = parsePermissionKey(key)

		item := PermissionItem{
			ID:          key,
			Description: "",
			Category:    category,
			Resource:    resource,
			Action:      action,
			Order:       "",
		}
		items = append(items, item)
	}

	return items
}

func NewPermissionRegistry() *PermissionRegistry {
	return &PermissionRegistry{
		nextPermissionID: 1,
		permMap:          make(map[string]PermissionID),
		revPermMap:       make(map[PermissionID]string),
	}
}

var permissionRegistry = NewPermissionRegistry()

func GetOrInitPermission(category, resource string, action string) PermissionID {
	key := GetSimplePermission(category, resource, action)
	return permissionRegistry.GetOrInitPermissionIDByKey(key)
}

func GetOrInitPermissionKeys(keys ...string) []PermissionID {
	out := []PermissionID{}
	for _, v := range keys {
		x := GetOrInitPermissionKey(v)
		out = append(out, x)
	}
	return out
}

func GetOrInitPermissionKey(key string) PermissionID {
	return permissionRegistry.GetOrInitPermissionIDByKey(key)
}

func MustRegisterPermissionByKey(key string) PermissionID {
	return permissionRegistry.MustGetPermissionIDByKey(key)
}

func MustRegisterPermissionByKeys(key []string) []PermissionID {
	v := []PermissionID{}
	for _, k := range key {
		v = append(v, permissionRegistry.MustGetPermissionIDByKey(k))
	}
	return v
}

func (pr *PermissionRegistry) MustGetPermissionID(category, resource string, action string) PermissionID {
	key := GetSimplePermission(category, resource, action)
	return pr.MustGetPermissionIDByKey(key)
}

func (pr *PermissionRegistry) MustGetPermissionKeyByID(id PermissionID) string {
	pr.locker.RLock()
	defer pr.locker.RUnlock()

	if key, exists := pr.revPermMap[id]; exists {
		return key
	}
	panic(errors.Errorf("invalid permission, id: %v not registered", id))
}

func (pr *PermissionRegistry) MustGetPermissionIDByKey(key string) PermissionID {
	pr.locker.RLock()
	defer pr.locker.RUnlock()

	if id, exists := pr.permMap[key]; exists {
		return id
	}
	panic(errors.Errorf("invalid permission, key: %v not registered", key))
}

func (pr *PermissionRegistry) GetOrInitPermissionIDByKey(key string) PermissionID {
	pr.locker.Lock()
	defer pr.locker.Unlock()

	if id, exists := pr.permMap[key]; exists {
		return id
	}
	id := pr.nextPermissionID
	pr.permMap[key] = id
	pr.revPermMap[id] = key
	pr.nextPermissionID++
	return id
}
