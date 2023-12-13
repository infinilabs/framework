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
	"infini.sh/framework/core/logger"
	"infini.sh/framework/core/util"
	"net/http"
)

func init() {
	api.HandleAPIFunc("/setting/logger", LoggingSettingAction)
	api.HandleAPIMethod(api.GET, "/setting/auth", authAPIHandler)
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

func authAPIHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	obj := util.MapStr{
		"enabled": api.IsAuthEnable(),
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(util.MustToJSONBytes(obj))
	w.WriteHeader(200)
}
