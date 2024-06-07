/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	"infini.sh/framework/core/util"
	"sync"
)

type AppSettings struct{
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
		if fv, ok := v.(func()interface{}); ok {
			ret[key] = fv()
		}else{
			ret[key] = v
		}
	}
	return ret
}

func GetAppSetting(key string) interface{} {
	v := appSettings.Get(key)
	if fv, ok := v.(func()interface{}); ok {
		return fv()
	}
	return v
}