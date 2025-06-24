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
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package stats

import (
	"fmt"
	"net/http"
	"regexp"
	"runtime"
	"strings"
	"sync"

	"github.com/segmentio/encoding/json"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/stats"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/bytebufferpool"
)

var space = []byte(" ")
var newline = []byte("\n")
var statsLock = sync.RWMutex{}

// StatsAction return stats information
func (handler SimpleStatsModule) StatsAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	var bytes []byte
	var err error

	metrics, err := stats.StatsMap()
	if err != nil {
		handler.WriteError(w, "stats is nil", 500)
		return
	}

	format := handler.GetParameter(req, "format")

	switch format {
	case "prometheus":
		handler.PrometheusStatsAction(w, req, ps)
		return
	default:
		statsLock.Lock()
		defer statsLock.Unlock()
		bytes, err = json.MarshalIndent(metrics, "", " ")
		if err != nil {
			handler.Error(w, err)
			return
		}
		handler.WriteJSONHeader(w)
		_, _ = handler.Write(w, bytes)
	}

	handler.WriteHeader(w, 200)
}

// StatsAction return stats information
func (handler SimpleStatsModule) PrometheusStatsAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {

	var err error
	metrics, err := stats.StatsMap()
	if err != nil {
		handler.WriteError(w, "stats is nil", 500)
		return
	}

	kv := util.Flatten(metrics, false)
	buffer := bytebufferpool.Get("stats")
	defer bytebufferpool.Put("stats", buffer)
	for k, v := range kv {
		_, _ = buffer.Write(util.UnsafeStringToBytes(util.PrometheusMetricReplacer.Replace(k)))
		_, _ = buffer.Write(util.UnsafeStringToBytes(fmt.Sprintf("{type=\"%v\", ip=\"%v\", name=\"%v\", id=\"%v\"}",
			global.Env().GetAppLowercaseName(),
			global.Env().SystemConfig.NodeConfig.IP,
			global.Env().SystemConfig.NodeConfig.Name,
			global.Env().SystemConfig.NodeConfig.ID,
		)))
		_, _ = buffer.Write(space)
		_, _ = buffer.Write(util.UnsafeStringToBytes(util.ToString(v)))
		_, _ = buffer.Write(newline)
	}
	handler.WriteTextHeader(w)
	_, _ = handler.Write(w, buffer.Bytes())

	handler.WriteHeader(w, 200)
}

func (handler SimpleStatsModule) GoroutinesAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	buf := make([]byte, 2<<20)
	n := runtime.Stack(buf, true)

	stacks := strings.Split(string(buf[:n]), "\n\n")
	grouped := make(map[string]int)
	patternMem, err := regexp.Compile("\\+?0x[\\d\\w]+")
	if err != nil {
		panic(err)
	}
	patternID, err := regexp.Compile("^goroutine \\d+")
	if err != nil {
		panic(err)
	}
	for _, stack := range stacks {
		newStack := patternMem.ReplaceAll([]byte(stack), []byte("_address_"))
		newStack = patternID.ReplaceAll(newStack, []byte("goroutine _id_"))
		grouped[string(newStack)]++
	}

	sorted := util.SortMapStrIntToKV(grouped)

	m := []util.MapStr{}
	for _, v := range sorted {
		o := util.MapStr{}
		o["goroutine"] = v.Key
		o["count"] = v.Value
		m = append(m, o)
	}

	handler.WriteJSON(w, m, 200)

}
