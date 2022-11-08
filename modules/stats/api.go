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

package stats

import (
	"fmt"
	"github.com/segmentio/encoding/json"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/stats"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/bytebufferpool"
	"net/http"
	"strings"
	"sync"
)

func getMapValue(mapData map[string]int, key string, defaultValue int32) int {
	data := mapData[key]
	return data
}

var space = []byte(" ")
var newline = []byte("\n")
var statsLock=sync.RWMutex{}

// StatsAction return stats information
func (handler SimpleStatsModule) StatsAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var bytes []byte
	var err error
	metricsJSON := stats.StatsAll()
	if metricsJSON==""{
		handler.WriteError(w,"stats is nil",500)
		return
	}
	metrics:=util.MapStr{}
	err=util.FromJSONBytes([]byte(metricsJSON),&metrics)
	if err!=nil{
		panic(err)
	}

	format :=handler.GetParameter(req,"format")

	switch format {
		case "prometheus":
			kv := util.Flatten(metrics, false)
			buffer := bytebufferpool.Get("stats")
			defer bytebufferpool.Put("stats", buffer)
			for k, v := range kv {
				buffer.Write(util.UnsafeStringToBytes(strings.ReplaceAll(k,".","_")))
				buffer.Write(util.UnsafeStringToBytes(fmt.Sprintf("{type=\"gateway\", ip=\"%v\", name=\"%v\", id=\"%v\"}",
					global.Env().SystemConfig.NodeConfig.IP,
					global.Env().SystemConfig.NodeConfig.Name,
					global.Env().SystemConfig.NodeConfig.ID,
					)))
				buffer.Write(space)
				buffer.Write(util.UnsafeStringToBytes(util.ToString(v)))
				buffer.Write(newline)
			}
			handler.WriteTextHeader(w)
			handler.Write(w, buffer.Bytes())
			break
		default:
			statsLock.Lock()
			defer statsLock.Unlock()
			bytes, err = json.MarshalIndent(metrics, "", " ")
			if err != nil {
				handler.Error(w, err)
				return
			}
			handler.WriteJSONHeader(w)
			handler.Write(w, bytes)
	}

	handler.WriteHeader(w, 200)
}
