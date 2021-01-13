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
	"infini.sh/framework/core/daemon"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/logger"
	"infini.sh/framework/core/module"
	"infini.sh/framework/core/stats"
	"infini.sh/framework/core/util"
	//"infini.sh/framework/plugins"
	defaultLog "log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/signal"
	"runtime"
	"runtime/pprof"
	"syscall"
)

type App struct {
	environment  *env.Env
	numCPU       int
	quitSignal   chan bool
	isDaemonMode bool
	isDebug      bool
	pidFile      string
	configFile   string
	logLevel     string
	cpuproFile   string
	memproFile   string
	httpprof     string
}

func NewApp(name, desc, ver, commit, buildDate, terminalHeader, terminalFooter string) *App {
	return &App{environment: env.NewEnv(name, desc, ver, commit, buildDate, terminalHeader, terminalFooter)}
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

type Options struct {
	EnableProfiling bool
}

func (app *App) Init(customFunc func()) {

	options := Options{
		EnableProfiling: true,
	}
	app.InitWithOptions(options, customFunc)
}
func (app *App) InitWithOptions(options Options, customFunc func()) {

	showversion:=flag.Bool("v", false, "version")
	flag.StringVar(&app.logLevel, "log", "info", "the log level,options:trace,debug,info,warn,error")
	flag.StringVar(&app.configFile, "config", app.environment.GetAppLowercaseName()+".yml", "the location of config file, default: "+app.environment.GetAppName()+".yml")
	flag.BoolVar(&app.isDaemonMode, "daemon", false, "run in background as daemon")
	flag.StringVar(&app.pidFile, "pidfile", "", "pidfile path (only for daemon mode)")
	flag.BoolVar(&app.isDebug, "debug", false, "run in debug mode, "+app.environment.GetAppName()+" will quit with panic error")
	flag.IntVar(&app.numCPU, "cpu", -1, "the number of CPUs to use")

	if options.EnableProfiling {
		flag.StringVar(&app.cpuproFile, "cpuprofile", "", "write cpu profile to this file")
		flag.StringVar(&app.memproFile, "memprofile", "", "write memory profile to this file")
		flag.StringVar(&app.httpprof, "pprof", "", "enable and setup pprof/expvar service, eg: localhost:6060 , the endpoint will be: http://localhost:6060/debug/pprof/ and http://localhost:6060/debug/vars")
	}

	flag.Parse()

	if *showversion{
		fmt.Println(app.environment.GetAppName(),app.environment.GetVersion(),app.environment.GetBuildDate(),app.environment.GetLastCommitHash())
		os.Exit(1)
	}


	defaultLog.SetOutput(logger.EmptyLogger{})

	logger.SetLogging(env.EmptyEnv(), app.logLevel)

	app.environment.IsDebug = app.isDebug

	app.environment.SetConfigFile(app.configFile)

	app.environment.Init()

	//put env into global registrar
	global.RegisterEnv(app.environment)

	logger.SetLogging(app.environment, app.logLevel)

	if options.EnableProfiling {

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
	}

	if customFunc != nil {
		customFunc()
	}
}

func (app *App) Start(setup func(), start func()) {

	if app.numCPU <= 0 {
		runtime.GOMAXPROCS(runtime.NumCPU())
	} else {
		runtime.GOMAXPROCS(app.numCPU)
	}

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

	//loading plugins
	//plugins.Discovery(app.environment.GetPluginDir())

	if setup != nil {
		setup()
	}

	if start != nil {
		start()
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
			fmt.Printf("\n[%s] got signal: %v, start shutting down\n", app.environment.GetAppCapitalName(), s.String())
			//wait workers to exit
			module.Stop()
			app.quitSignal <- true
		}
	}()

	log.Infof("%s now started.", app.environment.GetAppName())

	<-app.quitSignal
}

func (app *App) Shutdown() {
	//cleanup
	util.ClearInstanceLock()

	callbacks := global.ShutdownCallback()
	if callbacks != nil && len(callbacks) > 0 {
		for i, v := range callbacks {
			log.Trace("executing callback: ", i)
			v()
		}
	}

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

		buf := make([]byte, 1<<20)

		runtime.Stack(buf, app.environment.IsDebug)
		fmt.Printf("\n%s", buf)

	}

	util.SnapshotPersistID()

	log.Flush()
	logger.Flush()

	if app.environment.IsDebug {
		fmt.Println(string(*stats.StatsAll()))
	}

	if !app.isDaemonMode {
		log.Infof("%s now terminated.", app.environment.GetAppName())
		log.Flush()
		//print goodbye message
		fmt.Println(app.environment.GetGoodbyeMessage())
	}

	os.Exit(0)
}
