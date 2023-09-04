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

func getMapValue(mapData map[string]int, key string, defaultValue int32) int {
	data := mapData[key]
	return data
}

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
		handler.Write(w, bytes)
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
		buffer.Write(util.UnsafeStringToBytes(util.PrometheusMetricReplacer.Replace(k)))
		buffer.Write(util.UnsafeStringToBytes(fmt.Sprintf("{type=\"%v\", ip=\"%v\", name=\"%v\", id=\"%v\"}",
			global.Env().GetAppLowercaseName(),
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

	handler.WriteHeader(w, 200)
}

//func (handler SimpleStatsModule) GoroutinesAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
//	buf := make([]byte, 2<<20)
//	n := runtime.Stack(buf, true)
//
//	handler.WriteTextHeader(w)
//	handler.Write(w, buf[:n])
//	handler.WriteHeader(w, 200)
//}

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
