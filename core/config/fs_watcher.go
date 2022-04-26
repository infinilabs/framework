/* Copyright © INFINI Ltd. All rights reserved.
 * web: https://infinilabs.com
 * mail: hello#infini.ltd */

package config

import (
	log "github.com/cihub/seelog"
	"github.com/fsnotify/fsnotify"
	"infini.sh/framework/core/util"
	"sync"
)

type Watcher struct {
	path string
	watcher *fsnotify.Watcher
	callbacks []CallbackFunc
}

type CallbackFunc func(file string,op fsnotify.Op)

var fsWatchers map[string]*Watcher = map[string]*Watcher{}

func loadConfigFile(file string)*Config  {
	if util.SuffixStr(file,".yml")||util.SuffixStr(file,".yaml"){
		if !util.FileExists(file){
			return nil
		}
		v1,err:=LoadFile(file)
		if err!=nil{
			log.Error(err)
			return nil
		}
		return v1
	}
	return nil
}

func EnableWatcher(path string)  {
	if !util.FileExists(path){
		log.Debugf("path: %v not exists, skip watcher",path)
		return
	}
	AddPathToWatch(path, func(file string,op fsnotify.Op) {
		loadConfigFile(file)
	})

	log.Debugf("enable watcher on path: %v",path)

}

func AddPathToWatch(path string,callback CallbackFunc) {

	var err error
	watcher,ok:= fsWatchers[path]
	if ok{
		watcher.callbacks=append(watcher.callbacks,callback)
		return
	}

	fsWatcher, err := fsnotify.NewWatcher()
	if err!=nil{
		log.Error(err)
		return
	}

	watcher=&Watcher{
		path: path,
		watcher: fsWatcher,
		callbacks:[]CallbackFunc{callback},
	}

	fsWatchers[path]=watcher

	//cache:=util.NewCacheWithExpireOnAdd(5*time.Second,5)
	go func(watcher *Watcher) {
		for {
			select {
			case ev := <-fsWatcher.Events:
				{
					//TODO merge changes in 5 seconds
					log.Trace("config changed:",ev.String())

					for _,v:=range watcher.callbacks{
						v(ev.Name,ev.Op)
					}

					cfg:=loadConfigFile(ev.Name)
					if cfg==nil{
						continue
					}
					for k,v:=range notify{
						if cfg.HasField(k){
							currentCfg,err:=cfg.Child(k,-1)
							if err!=nil{
								log.Error(err)
								continue
							}
							// diff config
							previousCfg,_:=latestConfig[k]
							for _,f:=range v{
								f(previousCfg,currentCfg)
							}
							latestConfig[k]=currentCfg
						}
					}

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

var latestConfig =map[string]*Config{}

func StopWatchers() {
	for _,v:=range fsWatchers{
		if v.watcher!=nil{
			v.watcher.Close()
		}
	}
}

var notify = map[string][]func(pCfg,cCfg *Config){}
var cfgLocker=sync.RWMutex{}

func NotifyOnConfigSectionChange(configKey string,f func(pCfg,cCfg *Config))  {
	cfgLocker.Lock()
	defer cfgLocker.Unlock()

	v,ok:=notify[configKey]
	if !ok{
		v=[]func(pCfg,cCfg *Config){}
		notify[configKey]=v
	}
	v=append(v,f)
	notify[configKey]=v
}
