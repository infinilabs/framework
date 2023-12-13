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
	"infini.sh/framework/core/host"
	"infini.sh/framework/core/model"
	"infini.sh/framework/core/util"
	"net/http"
	"sort"
)

// Name return API
func (module *APIModule) Name() string {
	return "api"
}

func init() {
	api.HandleAPIMethod(api.GET, "/_whoami", whoisAPIHandler)
	api.HandleAPIMethod(api.GET, "/_version", versionAPIHandler)
	api.HandleAPIMethod(api.GET, "/_info", infoAPIHandler)
	api.HandleAPIMethod(api.GET, "/health", healthAPIHandler)
}

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
	hostInfo.Name, _, hostInfo.OS.Name, _, hostInfo.OS.Version, hostInfo.OS.Architecture, err = host.GetOSInfo()
	if err != nil {
		panic(err)
	}

	info := model.GetInstanceInfo()
	info.Host = &hostInfo
	w.Header().Set("Content-Type", "application/json")
	w.Write(util.MustToJSONBytes(info))
	w.WriteHeader(200)
}

func defaultHandler(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
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
}

func (module *APIModule) Setup() {
	//should not enable when UI module is enabled
	if !global.Env().SystemConfig.APIConfig.DisableAPIDirectory {
		p1 := global.Env().SystemConfig.APIConfig.APIDirectoryPath
		if p1 == "" {
			p1 = "/"
		}
		api.HandleAPIMethod(api.GET, p1, defaultHandler)
	}
}

func (module *APIModule) Start() error {
	api.StartAPI()
	return nil
}

func (module *APIModule) Stop() error {
	return nil
}

type APIModule struct {
	api.Handler
}
