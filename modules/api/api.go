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
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/model"
	"infini.sh/framework/core/util"
	"infini.sh/framework/modules/metrics"
	"net/http"
	"sort"
)

// Name return API
func (module *APIModule) Name() string {
	return "api"
}

const whoisAPI = "/_whoami"
const versionAPI = "/_version"
const infoAPI = "/_info"
const authAPI = "/setting/auth" //TODO, merge with /_settings
const healthAPI = "/health"

func whoisAPIHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	w.Write([]byte(global.Env().SystemConfig.APIConfig.NetworkConfig.GetPublishAddr()))
	w.Write([]byte("\n"))
	w.WriteHeader(200)
}

func versionAPIHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	w.Write([]byte(global.Env().GetVersion()))
	w.Write([]byte("\n"))
	w.WriteHeader(200)
}

func healthAPIHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {

	obj := util.MapStr{
		"status": global.Env().GetOverallHealth().ToString(),
	}

	services := global.Env().GetServicesHealth()
	if len(services) > 0 {
		obj["services"] = services
	}

	if global.Env().SetupRequired() {
		obj["setup_required"] = global.Env().SetupRequired()
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(util.MustToJSONBytes(obj))
	w.WriteHeader(200)
}

func infoAPIHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	hostInfo := model.HostInfo{
		OS: model.OSInfo{},
	}
	var err error
	hostInfo.Name, _, hostInfo.OS.Name, _, hostInfo.OS.Version, hostInfo.OS.Architecture, err = metrics.GetOSInfo()
	if err != nil {
		panic(err)
	}

	info := model.GetInstanceInfo()
	info.Host = &hostInfo
	w.Header().Set("Content-Type", "application/json")
	w.Write(util.MustToJSONBytes(info))
	w.WriteHeader(200)
}

func authAPIHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	obj := util.MapStr{
		"enabled": api.IsAuthEnable(),
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(util.MustToJSONBytes(obj))
	w.WriteHeader(200)
}

func init() {
	api.HandleAPIMethod(api.GET, versionAPI, versionAPIHandler)
	api.HandleAPIMethod(api.GET, infoAPI, infoAPIHandler)
	api.HandleAPIMethod(api.GET, authAPI, authAPIHandler)
	api.HandleAPIMethod(api.GET, healthAPI, healthAPIHandler)
}

// Start api server
func (module *APIModule) Setup() {
	api.HandleAPIMethod(api.GET, "/", func(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
		w.Write([]byte(global.Env().GetAppCapitalName()))
		w.Write([]byte(", "))
		w.Write([]byte(global.Env().GetVersion()))
		w.Write([]byte(", "))
		w.Write([]byte(global.Env().GetBuildNumber()))
		w.Write([]byte(", "))
		w.Write([]byte(util.FormatTime(global.Env().GetBuildDate())))
		w.Write([]byte(", "))
		w.Write([]byte(util.FormatTime(global.Env().GetEOLDate())))
		w.Write([]byte(", "))
		w.Write([]byte(global.Env().GetLastCommitHash()))
		w.Write([]byte("\n\n"))

		w.Write([]byte("API Directory:\n"))

		apis := util.GetMapKeys(api.APIs)
		sort.Strings(apis)

		for _, k := range apis {
			v, ok := api.APIs[k]
			if ok {
				w.Write([]byte(v.Key))
				w.Write([]byte("\t"))
				w.Write([]byte(v.Value))
				w.Write([]byte("\n"))
			}
		}

		w.WriteHeader(200)
	})
	api.HandleAPIMethod(api.GET, whoisAPI, whoisAPIHandler)
}

func (module *APIModule) Start() error {

	//API server
	api.StartAPI()

	return nil
}

// Stop api server
func (module *APIModule) Stop() error {
	return nil
}

// APIModule is used to start API server
type APIModule struct {
	api.Handler
}
