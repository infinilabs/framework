/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package api

import (
	log "github.com/cihub/seelog"
	"github.com/segmentio/encoding/json"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/logger"
	"net/http"
)

func init(){
	h := SettingHandler{}
	HandleAPIFunc("/setting/logger", h.LoggingSettingAction)
}

type SettingHandler struct {
	Handler
}

// LoggingSettingAction is the ajax request to update logging config
func (h SettingHandler) LoggingSettingAction(w http.ResponseWriter, req *http.Request) {
	if req.Method == GET.String() {

		cfg := logger.GetLoggingConfig()
		if cfg != nil {
			h.WriteJSON(w, cfg, 200)
		} else {
			h.Error500(w, "config not available")
		}

	} else if req.Method == PUT.String() || req.Method == POST.String() {
		body, err := h.GetRawBody(req)
		if err != nil {
			log.Error(err)
			h.Error500(w, "config update failed")
			return
		}

		configStr := string(body)

		cfg := config.LoggingConfig{}

		err = json.Unmarshal([]byte(configStr), &cfg)

		if err != nil {
			h.Error(w, err)

		}

		log.Debug("receive new settings:", configStr)

		var (
			appName = global.Env().GetAppLowercaseName()
			baseDir = global.Env().GetLogDir()
		)
		logger.SetLogging(&cfg, appName, baseDir)

		h.WriteJSON(w, map[string]interface{}{"success": true}, http.StatusOK)

	}
}

