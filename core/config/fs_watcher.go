/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package config

import (
	"runtime"
	"sync"
	"time"

	log "github.com/cihub/seelog"
	"github.com/fsnotify/fsnotify"
	"infini.sh/framework/core/util"
)

type Watcher struct {
	path      string
	watcher   *fsnotify.Watcher
	callbacks []CallbackFunc
}

type CallbackFunc func(file string, op fsnotify.Op)

var fsWatchers map[string]*Watcher = map[string]*Watcher{}

func loadConfigFile(file string) *Config {
	if util.SuffixStr(file, ".yml") || util.SuffixStr(file, ".yaml") {
		if !util.FileExists(file) {
			return nil
		}
		v1, err := LoadFile(file)
		if err != nil {
			log.Error(err)
			return nil
		}
		return v1
	}
	return nil
}

var validExtensions = []string{".yml", ".yaml"}

func SetValidExtension(v []string)  {
	validExtensions=v
}

func EnableWatcher(path string) {
	if !util.FileExists(path) {
		log.Debugf("path: %v not exists, skip watcher", path)
		return
	}
	AddPathToWatch(path, func(file string, op fsnotify.Op) {
		log.Debug("reload file: ", file, ",", op.String())
	})

	log.Debugf("enable watcher on path: %v", path)
}

var watcherLock = sync.Once{}
var watcherIsRunning = false

// event bus
var events chan fsnotify.Event = make(chan fsnotify.Event, 10)

// AddPathToWatch should only be called for configuration paths
func AddPathToWatch(path string, callback CallbackFunc) {

	var err error
	watcher, ok := fsWatchers[path]
	if ok {
		watcher.callbacks = append(watcher.callbacks, callback)
		return
	}

	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Error(err)
		return
	}

	watcher = &Watcher{
		path:      path,
		watcher:   fsWatcher,
		callbacks: []CallbackFunc{callback},
	}

	fsWatchers[path] = watcher

	watcherLock.Do(func() {
		if watcherIsRunning {
			return
		}
		watcherIsRunning = true

		//handle config reload
		go func() {
			defer func() {
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
					log.Trace("error on handle configs,", v)
				}
			}()

			var ev fsnotify.Event
			var ok bool
			for {
				ev, ok = <-events
				if !ok {
					continue
				}

				if ev.Op==fsnotify.Chmod{
					continue
				}

				if ev.Op==fsnotify.Rename{
					continue
				}

				if len(validExtensions)>0 && !util.SuffixAnyInArray(ev.Name,validExtensions){
					continue
				}

				log.Trace("2 seconds wait, on:", ev.String())
				time.Sleep(2 * time.Second)
				log.Trace("2 seconds out, on:", ev.String())

				// AddPathToWatch

				for _, v := range watcher.callbacks {
					v(ev.Name, ev.Op)
				}

				// NotifyOnConfigChange

				for _, v := range configCallbacks {
					v(ev)
				}

				// NotifyOnConfigSectionChange

				cfg := loadConfigFile(ev.Name)
				if cfg == nil {
					continue
				}

				for k, v := range sectionCallbacks {
					if cfg.HasField(k) {
						currentCfg, err := cfg.Child(k, -1)
						if err != nil {
							log.Error(err)
							continue
						}
						// diff config
						previousCfg, _ := latestConfig[k]
						for _, f := range v {
							f(previousCfg, currentCfg)
						}
						latestConfig[k] = currentCfg
					}
				}
			}
		}()
	})

	//handle events
	go func(watcher *Watcher) {
		defer func() {
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
				log.Trace("error on handle configs,", v)
			}
		}()

		//handle events merge
		for {
			select {
			case ev := <-fsWatcher.Events:
				{
					if util.SuffixStr(ev.Name, "~") {
						log.Trace("skip temp file:", ev.String())
						continue
					}
					log.Trace("config changed:", ev.String())
					events <- ev
				}
			case err := <-fsWatcher.Errors:
				{
					log.Debug("error : ", err)
					return
				}
			}
		}

	}(watcher)

	err = fsWatcher.Add(path)
	if err != nil {
		log.Error(err)
		return
	}
}

var latestConfig = map[string]*Config{}

func StopWatchers() {

	log.Trace("stopping watchers")
	for i, v := range fsWatchers {
		log.Trace("stopping watcher: ", i)
		if v.watcher != nil {
			v.watcher.Close()
		}
		log.Trace("stopped watcher: ", i)
	}

	log.Trace("start closing watcher events channel")
	close(events)
}

var sectionCallbacks = map[string][]func(pCfg, cCfg *Config){}
var configCallbacks = []func(fsnotify.Event){}
var cfgLocker = sync.RWMutex{}

// NotifyOnConfigSectionChange will trigger callback when any configuration file change detected and
// configKey present in the changed file
func NotifyOnConfigSectionChange(configKey string, f func(pCfg, cCfg *Config)) {
	cfgLocker.Lock()
	defer cfgLocker.Unlock()

	v, ok := sectionCallbacks[configKey]
	if !ok {
		v = []func(pCfg, cCfg *Config){}
		sectionCallbacks[configKey] = v
	}
	v = append(v, f)
	sectionCallbacks[configKey] = v
}

// NotifyOnConfigChange will trigger callback when any configuration file change detected
func NotifyOnConfigChange(f func(fsnotify.Event)) {
	cfgLocker.Lock()
	defer cfgLocker.Unlock()

	configCallbacks = append(configCallbacks, f)
}
