/*
Copyright 2016 Medcl (m AT medcl.net)

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

   http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package api

import (
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/global"
	"net/http"
)

// Name return API
func (module APIModule) Name() string {
	return "API"
}

const whoisAPI = "/_framework/api/_whoami"


// Start api server
func (module APIModule) Setup(cfg *config.Config) {
	api.HandleAPIMethod(api.GET, whoisAPI, func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		w.Write([]byte(global.Env().SystemConfig.APIConfig.NetworkConfig.GetPublishAddr()))
		w.WriteHeader(200)
	})

	//API server
	api.StartAPI(cfg)
}
func (module APIModule) Start() error {

	return nil
}

// Stop api server
func (module APIModule) Stop() error {
	return nil
}

// APIModule is used to start API server
type APIModule struct {
}
