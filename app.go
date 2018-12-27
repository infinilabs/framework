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

package framework

import (
	"expvar"
	_ "expvar"
	"flag"
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/infinitbyte/framework/core/daemon"
	"github.com/infinitbyte/framework/core/env"
	"github.com/infinitbyte/framework/core/global"
	"github.com/infinitbyte/framework/core/logger"
	"github.com/infinitbyte/framework/core/module"
	"github.com/infinitbyte/framework/core/stats"
	"github.com/infinitbyte/framework/core/util"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"
)

type App struct {
	environment  *env.Env
	quitSignal   chan bool
	isDaemonMode bool
	isDebug      bool
	pidFile      string
	configFile   string
	logLevel     string
	cpuproFile   string
	memproFile   string
	httpprof     string
	logDir       string
}

func NewApp(name, desc, ver, commit, buildDate, terminalHeader, terminalFooter string) App {
	return App{environment: env.NewEnv(name, desc, ver, commit, buildDate, terminalHeader, terminalFooter)}
}

// report expvar and all metrics
func (app *App) metricsHandler(w http.ResponseWriter, r *http.Request) {
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

func (app *App) Init(customFunc func()) {
	flag.StringVar(&app.logLevel, "log", "info", "the log level,options:trace,debug,info,warn,error")
	flag.StringVar(&app.configFile, "config", app.environment.GetAppName()+".yml", "the location of config file, default: "+app.environment.GetAppName()+".yml")
	flag.BoolVar(&app.isDaemonMode, "daemon", false, "run in background as daemon")
	flag.BoolVar(&app.isDebug, "debug", false, "run in debug mode, "+app.environment.GetAppName()+" will quit with panic error")
	flag.StringVar(&app.pidFile, "pidfile", "", "pidfile path (only for daemon mode)")
	flag.StringVar(&app.cpuproFile, "cpuprofile", "", "write cpu profile to this file")
	flag.StringVar(&app.memproFile, "memprofile", "", "write memory profile to this file")
	flag.StringVar(&app.httpprof, "pprof", "", "enable and setup pprof/expvar service, eg: localhost:6060 , the endpoint will be: http://localhost:6060/debug/pprof/ and http://localhost:6060/debug/vars")

	flag.StringVar(&app.logDir, "log_path", "log", "the log path")

	flag.Parse()

	logger.SetLogging(env.EmptyEnv(), app.logLevel, app.logDir)

	app.environment.IsDebug = app.isDebug

	app.environment.SetConfigFile(app.configFile)

	app.environment.Init()

	//put env into global registrar
	global.RegisterEnv(app.environment)

	logger.SetLogging(app.environment, app.logLevel, app.logDir)

	//profile options
	if app.httpprof != "" {
		go func() {
			log.Infof("pprof listen at: http://%s/debug/pprof/", app.httpprof)
			mux := http.NewServeMux()

			// register pprof handler
			mux.HandleFunc("/debug/pprof/", func(w http.ResponseWriter, r *http.Request) {
				http.DefaultServeMux.ServeHTTP(w, r)
			})

			// register metrics handler
			mux.HandleFunc("/debug/vars", app.metricsHandler)

			endpoint := http.ListenAndServe(app.httpprof, mux)
			log.Debug("stop pprof server: %v", endpoint)
		}()
	}

	if app.cpuproFile != "" {
		f, err := os.Create(app.cpuproFile)
		if err != nil {
			panic(err)
		}
		pprof.StartCPUProfile(f)
		defer pprof.StopCPUProfile()
	}

	if app.memproFile != "" {
		if app.memproFile != "" {
			f, err := os.Create(app.memproFile)
			if err != nil {
				panic(err)
			}
			pprof.WriteHeapProfile(f)
			f.Close()
		}
	}

	if customFunc != nil {
		log.Trace("start execute custom init func")
		customFunc()
		log.Trace("end execute custom init func")
	}
}

func (app *App) Start(setup func(), start func()) {
	runtime.GOMAXPROCS(runtime.NumCPU())

	fmt.Println(app.environment.GetWelcomeMessage())

	//daemon
	if app.isDaemonMode {
		if runtime.GOOS == "darwin" || runtime.GOOS == "linux" {
			log.Trace(app.environment.GetAppName(), " enter daemon mode")
			runtime.LockOSThread()
			context := new(daemon.Context)
			if app.pidFile != "" {
				context.PidFileName = app.pidFile
				context.PidFilePerm = 0644
			}
			child, _ := context.Reborn()

			if child != nil {
				fmt.Printf("[%s] started in background, pid: %v\n", app.environment.GetAppCapitalName(), os.Getpid()+1)
				return
			}
			defer context.Release()

			runtime.UnlockOSThread()
		} else {
			fmt.Println("daemon mode only available on linux and darwin")
		}
	}

	//check instance lock
	util.CheckInstanceLock(app.environment.GetWorkingDir())

	//set path to persist id
	util.RestorePersistID(app.environment.GetWorkingDir())

	if setup != nil {
		log.Trace("start execute custom setup func")
		setup()
		log.Trace("end execute custom setup func")
	}

	if start != nil {
		log.Trace("start execute custom start func")
		start()
		log.Trace("end execute custom start func")
	}

	app.quitSignal = make(chan bool)

	//handle exit event
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc,
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGQUIT,
		os.Interrupt)

	go func() {
		s := <-sigc
		if s == os.Interrupt || s.(os.Signal) == syscall.SIGINT || s.(os.Signal) == syscall.SIGTERM ||
			s.(os.Signal) == syscall.SIGKILL || s.(os.Signal) == syscall.SIGQUIT {
			fmt.Printf("\n[%s] got signal:%v, start shutting down\n", app.environment.GetAppCapitalName(), s.String())
			//wait workers to exit
			module.Stop()
			app.quitSignal <- true
		}
	}()

	<-app.quitSignal
}

func (app *App) Shutdown() {
	//cleanup
	util.ClearInstanceLock()

	if !global.Env().IsDebug {
		if r := recover(); r != nil {
			if r == nil {
				return
			}
			var v string
			switch r.(type) {
			case error:
				v = r.(error).Error()
			case runtime.Error:
				v = r.(runtime.Error).Error()
			case string:
				v = r.(string)
			}
			log.Error("shutdown: ", v)
		}
	}

	util.SnapshotPersistID()

	log.Flush()
	logger.Flush()

	if app.environment.IsDebug {
		fmt.Println(string(*stats.StatsAll()))
	}

	if !app.isDaemonMode {
		//print goodbye message
		fmt.Println(app.environment.GetGoodbyeMessage())
	}

	os.Exit(0)
}
