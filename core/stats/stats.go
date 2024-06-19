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
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/util"
	"strings"
	"sync"
	"time"
)

type StatsInterface interface {

	Increment(category, key string)

	IncrementBy(category, key string, value int64)

	Decrement(category, key string)

	DecrementBy(category, key string, value int64)

	Absolute(category, key string, value int64)

	Timing(category, key string, v int64)



	Gauge(category, key string, v int64)

	Stat(category, key string) int64

	StatsAll() string

	//record the last timestamp for specify operation
	RecordTimestamp(category, key string, value time.Time)
	//get the last timestamp for specify operation
	GetTimestamp(category, key string)(time.Time, error)
}

var handlers = []StatsInterface{}

func JoinArray(array []string, delimiter string) string {
	if len(array) > 10 {
		return strings.Join(array, delimiter)
	}

	var str string
	x := len(array) - 1
	for i, v := range array {
		str += v
		if i < x {
			str += delimiter
		}
	}
	return str
}

//record the last timestamp for specify operation
func TimestampNow(category string, key ...string) {
	t:=util.GetLowPrecisionCurrentTime()
	Timestamp(category, JoinArray(key, "."), t)
}

func Timestamp(category, key string, value time.Time) {
	if len(handlers) == 0 {
		return
	}

	for _, v := range handlers {
		v.RecordTimestamp(category, key, value)
	}
}

func GetTimestamp(category string, key ...string) *time.Time {
	if len(handlers) == 0 {
		return nil
	}

	for _, v := range handlers {
		o,err:=v.GetTimestamp(category, JoinArray(key, "."))
		if err==nil{
			return &o
		}
	}
	return nil
}

func Increment(category string, key ...string) {
	if len(handlers) == 0 {
		return
	}

	IncrementBy(category, JoinArray(key, "."), 1)
}

func IncrementBy(category, key string, value int64) {
	for _, v := range handlers {
		v.IncrementBy(category, key, value)
	}
}

func Decrement(category, key string) {
	if len(handlers) == 0 {
		return
	}

	DecrementBy(category, key, 1)
}

func DecrementBy(category, key string, value int64) {
	for _, v := range handlers {
		v.DecrementBy(category, key, value)
	}
}

func Absolute(category, key string, value int64) {
	for _, v := range handlers {
		v.Absolute(category, key, value)
	}
}

func Timing(category, key string, value int64) {
	for _, v := range handlers {
		v.Timing(category, key, value)
	}
}

func Gauge(category, key string, value int64) {
	for _, v := range handlers {
		v.Gauge(category, key, value)
	}
}

func Stat(category, key string) int64 {
	for _, v := range handlers {
		b := v.Stat(category, key)
		if b > 0 {
			return b
		}
	}
	return 0
}

func statsAll() string{
	for _, v := range handlers {
		b := v.StatsAll()
		if b != "" {
			return b
		}
	}
	return ""
}

var registeredStats = map[string]func()interface{}{}
var registerLock=sync.Mutex{}
func RegisterStats(statsKey string,callback func()interface{})  {
	registerLock.Lock()
	registeredStats[statsKey]=callback
	registerLock.Unlock()
}

func StatsMap() (util.MapStr,error) {
	var err error
	metricsJSON := statsAll()
	if metricsJSON==""{
		return nil,errors.New("invalid stats")
	}
	metrics:=util.MapStr{}
	err=util.FromJSONBytes([]byte(metricsJSON),&metrics)
	if err!=nil{
		return nil,err
	}

	if len(registeredStats)>0{
		registerLock.Lock()
		for k,v:=range registeredStats{
			metrics[k]=v()
		}
		registerLock.Unlock()
	}

	return metrics,nil
}

func Register(h StatsInterface) {
	handlers = append(handlers, h)
}
