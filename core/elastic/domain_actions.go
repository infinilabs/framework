/*
Copyright Medcl (m AT medcl.net)

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

package elastic

import (
	"fmt"
	log "github.com/cihub/seelog"
	"sync"
)

var apis = sync.Map{}
var cfgs = sync.Map{}
var metas = sync.Map{}
var hosts=sync.Map{}

func RegisterInstance(elastic string, cfg ElasticsearchConfig, handler API) {
	apis.Store(elastic,handler)
	cfgs.Store(elastic,&cfg)
}

func GetOrInitHost(host string)(*NodeAvailable)  {
	v:= NodeAvailable{Available: true}
	v1,loaded:=hosts.LoadOrStore(host,&v)
	if loaded{
		return v1.(*NodeAvailable)
	}
	return &v
}

func RemoveInstance(elastic string){
	cfgs.Delete(elastic)
	apis.Delete(elastic)
	metas.Delete(elastic)
}

func GetConfig(k string) *ElasticsearchConfig {
	if k == "" {
		panic(fmt.Errorf("elasticsearch config undefined"))
	}
	v, ok := cfgs.Load(k)
	if !ok {
		panic(fmt.Sprintf("elasticsearch config [%v] was not found", k))
	}
	return v.(*ElasticsearchConfig)
}

func GetOrInitMetadata(cfg *ElasticsearchConfig) *ElasticsearchMetadata {
	v:=GetMetadata(cfg.ID)
	if v==nil{
		v=&ElasticsearchMetadata{Config: cfg}
		v.Init(false)
		SetMetadata(cfg.ID,v)
	}
	return v
}

func GetMetadata(k string) *ElasticsearchMetadata {
	if k == "" {
		panic(fmt.Errorf("elasticsearch metata undefined"))
	}

	v, ok := metas.Load(k)
	if !ok {
		log.Debug(fmt.Sprintf("elasticsearch metadata [%v] was not found", k))
		return nil
	}
	 x,ok:=v.(*ElasticsearchMetadata)
	 return x
}

func GetClient(k string) API {
	if k == "" {
		panic(fmt.Errorf("elasticsearch config undefined"))
	}

	v, ok := apis.Load(k)
	if ok {
		f,ok:=v.(API)
		if ok{
			return f
		}
	}

	panic(fmt.Sprintf("elasticsearch client [%v] was not found", k))
}

func WalkMetadata(walkFunc func(key, value interface{}) bool){
	metas.Range(walkFunc)
}

func WalkConfigs(walkFunc func(key, value interface{})bool) {
	 cfgs.Range(walkFunc)
}

func WalkHosts(walkFunc func(key, value interface{})bool) {
	 hosts.Range(walkFunc)
}

func SetMetadata(k string, v *ElasticsearchMetadata) {
	metas.Store(k,v)
}


func GetAvailableHost(cluster string)string  {
	meta:=GetMetadata(cluster)
	for _,v:=range meta.Nodes{
		if IsHostAvailable(v.Http.PublishAddress){
			return v.Http.PublishAddress
		}
	}
	return meta.Config.Endpoint
}

func IsHostAvailable(endpoint string)bool {
	info,ok:=hosts.Load(endpoint)
	if ok{
		return info.(*NodeAvailable).Available
	}
	log.Warnf("available info for [%v] was not found",endpoint)
	return false
}
