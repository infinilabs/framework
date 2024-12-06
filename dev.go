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

//go:build dev
// +build dev

/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package framework

import (
	"expvar"
	"flag"
	"fmt"
	"github.com/arl/statsviz"
	log "github.com/cihub/seelog"
	"github.com/rubyniu105/framework/core/global"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"runtime/pprof"
)

var cpuproFile string
var memproFile string
var httpprof string

// report expvar and all metrics
func metricsHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	first := true
	report := func(key string, value interface{}) {
		if !first {
			fmt.Fprintf(w, ",\n")
		}
		first = false
		if str, ok := value.(string); ok {
			fmt.Fprintf(w, "%q: %q", key, str)
		} else {
			fmt.Fprintf(w, "%q: %v", key, value)
		}
	}

	fmt.Fprintf(w, "{\n")
	expvar.Do(func(kv expvar.KeyValue) {
		report(kv.Key, kv.Value)
	})
	fmt.Fprintf(w, "\n}\n")
}

func init() {

	fmt.Println("[WARNING] THIS IS IN DEVELOPMENT MODE.")

	debugFlagInitFunc = func() {
		flag.StringVar(&cpuproFile, "cpu-profile", "", "write cpu profile to this file")
		flag.StringVar(&memproFile, "mem-profile", "", "write memory profile to this file")
		flag.StringVar(&httpprof, "pprof", "", "enable and setup pprof/expvar service, eg: localhost:6060 , the endpoint will be: http://localhost:6060/debug/pprof/ and http://localhost:6060/debug/vars")

	}

	debugInitFunc = func() {

		//profile options
		if httpprof != "" {
			go func() {

				defer func() {
					if !global.Env().IsDebug {
						if r := recover(); r != nil {
							var v string
							switch r.(type) {
							case error:
								v = r.(error).Error()
							case runtime.Error:
								v = r.(runtime.Error).Error()
							case string:
								v = r.(string)
							}
							log.Error("error to serve httpprof,", v)
						}
					}
				}()

				log.Infof("pprof listen at: http://%s/debug/pprof/", httpprof)
				mux := http.NewServeMux()

				// http://localhost:6060/debug/statsviz/
				statsviz.Register(mux)

				log.Infof("statsviz listen at: http://%s/debug/statsviz/", httpprof)

				// register pprof handler
				mux.HandleFunc("/debug/pprof/", func(w http.ResponseWriter, r *http.Request) {
					http.DefaultServeMux.ServeHTTP(w, r)
				})

				// register metrics handler
				mux.HandleFunc("/debug/vars", metricsHandler)

				endpoint := http.ListenAndServe(httpprof, mux)
				log.Debug("stop pprof server: %v", endpoint)
			}()
		}

		if cpuproFile != "" {
			f, err := os.Create(cpuproFile)
			if err != nil {
				panic(err)
			}
			pprof.StartCPUProfile(f)
			defer pprof.StopCPUProfile()
		}

		if memproFile != "" {
			if memproFile != "" {
				f, err := os.Create(memproFile)
				if err != nil {
					panic(err)
				}
				pprof.WriteHeapProfile(f)
				f.Close()
			}
		}
	}

}
