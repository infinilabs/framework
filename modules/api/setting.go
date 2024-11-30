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
	log "github.com/cihub/seelog"
	"github.com/segmentio/encoding/json"
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/logging/logger"
	"infini.sh/framework/core/util"
	"net/http"
)

func init() {
	api.HandleAPIFunc("/setting/logger", LoggingSettingAction)
	api.HandleAPIMethod(api.GET, "/setting/application", appSettingsAPIHandler)
}

// LoggingSettingAction is the ajax request to update logging config
func LoggingSettingAction(w http.ResponseWriter, req *http.Request) {
	if req.Method == api.GET.String() {
		cfg := logger.GetLoggingConfig()
		if cfg != nil {
			api.DefaultAPI.WriteJSON(w, cfg, 200)
		} else {
			api.DefaultAPI.Error500(w, "config not available")
		}
	} else if req.Method == api.PUT.String() || req.Method == api.POST.String() {
		body, err := api.DefaultAPI.GetRawBody(req)
		if err != nil {
			panic(err)
		}
		configStr := string(body)
		cfg := config.LoggingConfig{}
		err = json.Unmarshal([]byte(configStr), &cfg)
		if err != nil {
			panic(err)
		}
		log.Debug("receive new settings:", configStr)
		var (
			appName = global.Env().GetAppLowercaseName()
			baseDir = global.Env().GetLogDir()
		)
		logger.SetLogging(&cfg, appName, baseDir)
		api.DefaultAPI.WriteJSON(w, map[string]interface{}{"success": true}, http.StatusOK)
	}
}

func appSettingsAPIHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	obj := util.MapStr{
		"auth_enabled": api.IsAuthEnable(),
	}
	appSettings := api.GetAppSettings()
	obj.Merge(appSettings)
	w.Header().Set("Content-Type", "application/json")
	w.Write(util.MustToJSONBytes(obj))
	w.WriteHeader(200)
}
