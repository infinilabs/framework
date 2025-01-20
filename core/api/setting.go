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

/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	"infini.sh/framework/core/util"
	"sync"
)

type AppSettings struct {
	util.MapStr
	mu sync.RWMutex
}

func (settings *AppSettings) Add(key string, v interface{}) {
	settings.mu.Lock()
	defer settings.mu.Unlock()
	settings.MapStr[key] = v
}
func (settings *AppSettings) GetSettingsMap() util.MapStr {
	appSettings.mu.RLock()
	defer appSettings.mu.RUnlock()
	return settings.Clone()
}
func (settings *AppSettings) Get(key string) interface{} {
	appSettings.mu.RLock()
	defer appSettings.mu.RUnlock()
	return settings.MapStr[key]
}

var appSettings = AppSettings{
	MapStr: util.MapStr{},
}

func RegisterAppSetting(key string, v interface{}) {
	appSettings.Add(key, v)
}

func GetAppSettings() util.MapStr {
	ret := util.MapStr{}
	settings := appSettings.GetSettingsMap()
	for key, v := range settings {
		if fv, ok := v.(func() interface{}); ok {
			ret[key] = fv()
		} else {
			ret[key] = v
		}
	}
	return ret
}

func GetAppSetting(key string) interface{} {
	v := appSettings.Get(key)
	if fv, ok := v.(func() interface{}); ok {
		return fv()
	}
	return v
}
