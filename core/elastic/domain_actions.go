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
	"crypto/tls"
	"fmt"
	uri "net/url"
	"strings"
	"sync"
	"time"

	log "github.com/cihub/seelog"
	"github.com/dgraph-io/ristretto"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/rate"
	"infini.sh/framework/core/stats"
	"infini.sh/framework/core/util"
	"infini.sh/framework/lib/fasthttp"
)

// 保存了elastic.API(client)
var apis = sync.Map{}

// 保存了ElasticsearchConfig
var cfgs = sync.Map{}

// 存储了es的metadata(es配置+缓存配置)
var metas = sync.Map{}

var hosts = sync.Map{}

/*
*
注册elastic实例,初始化elastic metadata并保存到metas(sync.Map{})

cfg.ID = cfg.Name = 集群名称

把handle(API)保存到apis(sync.Map)

把ElasticsearchConfig保存到cfgs(sync.Map)
*/
func RegisterInstance(cfg ElasticsearchConfig, handler API) {

	if cfg.ID == "" {
		if cfg.Name == "" {
			panic(errors.Errorf("invalid elasticsearch config, id and name is not set, %v", cfg))
		}
		cfg.ID = cfg.Name
	}
	oldCfg, exists := cfgs.Load(cfg.ID)

	if exists {
		//if config no change, skip init
		if util.ToJson(cfg, false) == util.ToJson(oldCfg, false) {
			log.Trace("cfg no change, skip init, ", oldCfg)
			return
		}
	}

	UpdateClient(cfg, handler)
	UpdateConfig(cfg)

	if exists && oldCfg != nil {
		InitMetadata(&cfg, true)
	}
}

func UpdateConfig(cfg ElasticsearchConfig) {
	cfgs.Store(cfg.ID, &cfg)
}

func UpdateClient(cfg ElasticsearchConfig, handler API) {
	apis.Store(cfg.ID, handler)
}

/*
*
根据IP地址，返回可用的Node信息。 这里的Node并不包含es配置信息。 仅仅是集群的状态(是否可用)

如果根据IP从hosts(sync.Map)查询不到Node，则根据入参(host string,clusterID string)创建Node并保存
*/
func GetOrInitHost(host string, clusterID string) *NodeAvailable {
	if host == "" {
		return nil
	}

	//unify host
	if util.ContainStr(host, "localhost") {
		host = strings.Replace(host, "localhost", "127.0.0.1", -1)
	} else if util.ContainStr(host, "::1") {
		host = strings.Replace(host, "::1", "127.0.0.1", -1)
	}

	v1, loaded := hosts.Load(host)
	if loaded {
		return v1.(*NodeAvailable)
	} else {
		log.Tracef("init host: %v", host)
		v1 = &NodeAvailable{Host: host, available: util.TestTCPAddress(host, time.Second), ClusterID: clusterID}
		hosts.Store(host, v1)
	}
	return v1.(*NodeAvailable)
}

func RemoveInstance(elastic string) {
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

func GetConfigNoPanic(k string) *ElasticsearchConfig {
	if k == "" {
		panic(fmt.Errorf("elasticsearch config undefined"))
	}
	v, ok := cfgs.Load(k)
	if !ok {
		return nil
	}
	return v.(*ElasticsearchConfig)
}

var versions = map[string]int{}
var versionLock = sync.RWMutex{}

func (c *ElasticsearchConfig) ParseMajorVersion() int {
	if c.Version != "" {
		vs := strings.Split(c.Version, ".")
		n, err := util.ToInt(vs[0])
		if err != nil {
			panic(err)
		}
		return n
	}
	return -1
}

func (c *ElasticsearchConfig) GetAnyEndpoint() string {
	if c.Endpoint != "" {
		return c.Endpoint
	}
	if c.Endpoints != nil && len(c.Endpoints) > 0 {
		return c.Endpoints[0]
	}

	if c.Host != "" {
		return fmt.Sprintf("%s://%s", c.Schema, c.Host)
	}

	if c.Hosts != nil && len(c.Hosts) > 0 {
		return fmt.Sprintf("%s://%s", c.Schema, c.Hosts[0])
	}

	panic(fmt.Errorf("no endpoint was not found in config [%v] ", c.ID))
}

func (meta *ElasticsearchMetadata) GetMajorVersion() int {

	versionLock.RLock()
	esMajorVersion, ok := versions[meta.Config.ID]
	versionLock.RUnlock()

	if !ok {
		versionLock.Lock()
		defer versionLock.Unlock()

		v := meta.Config.ParseMajorVersion()
		if v > 0 {
			versions[meta.Config.ID] = v
			return v
		}

		esMajorVersion = GetClient(meta.Config.ID).GetMajorVersion()
		if esMajorVersion > 0 {
			versions[meta.Config.ID] = esMajorVersion
		}
	}

	return esMajorVersion
}

/*
*
初始化: ElasticsearchMetadata = ElasticsearchConfig + cache

并保存到metas(sync.Map)
*/
func InitMetadata(cfg *ElasticsearchConfig, defaultHealth bool) *ElasticsearchMetadata {
	v := &ElasticsearchMetadata{Config: cfg}

	if cfg.MetadataCacheEnabled {
		cache, err := ristretto.NewCache(&ristretto.Config{
			NumCounters: 1e5,     // Num keys to track frequency of (10M). 10,0000
			MaxCost:     1000000, //cfg.MaxCachedSize, // Maximum cost of cache (1GB).
			BufferItems: 64,      // Number of keys per Get buffer.
			Metrics:     false,
		})
		if err != nil {
			panic(err)
		}
		v.cache = cache
	}

	v.Init(defaultHealth)
	SetMetadata(cfg.ID, v)
	return v
}

func GetOrInitMetadata(cfg *ElasticsearchConfig) *ElasticsearchMetadata {
	if cfg.ID == "" {
		if cfg.Name == "" {
			panic(errors.Errorf("invalid elasticsearch config, id and name is not set, %v", cfg))
		}
		cfg.ID = cfg.Name
	}
	v := GetMetadata(cfg.ID)
	if v == nil {
		v = InitMetadata(cfg, false)
	}
	return v
}

func GetMetadata(k string) *ElasticsearchMetadata {
	if k == "" {
		panic(fmt.Errorf("elasticsearch id is nil"))
	}

	v, ok := metas.Load(k)
	if !ok {
		log.Debug(fmt.Sprintf("elasticsearch metadata [%v] was not found", k))
		return nil
	}
	x, ok := v.(*ElasticsearchMetadata)
	return x
}

func GetClient(k string) API {
	if k == "" {
		panic(fmt.Errorf("elasticsearch id is nil"))
	}

	v, ok := apis.Load(k)
	if ok {
		f, ok := v.(API)
		if ok {
			return f
		}
	}

	panic(fmt.Sprintf("elasticsearch client [%v] was not found", k))
}

// add by ck
func GetClientNoPanic(k string) API {
	if k == "" {
		panic(fmt.Errorf("elasticsearch id is nil"))
	}

	v, ok := apis.Load(k)
	if ok {
		f, ok := v.(API)
		if ok {
			return f
		}
	}
	return nil
}

// 最后返回的为判断是否继续 walk
func WalkMetadata(walkFunc func(key, value interface{}) bool) {
	metas.Range(walkFunc)
}

func WalkConfigs(walkFunc func(key, value interface{}) bool) {
	cfgs.Range(walkFunc)
}

func WalkHosts(walkFunc func(key, value interface{}) bool) {
	hosts.Range(walkFunc)
}

func RemoveHostsByClusterID(clusterID string) {
	hosts.Range(func(key, value any) bool {
		if v, ok := value.(*NodeAvailable); ok && v.ClusterID == clusterID {
			hosts.Delete(key)
		}
		return true
	})
}

func SetMetadata(k string, v *ElasticsearchMetadata) {
	metas.Store(k, v)
}

func IsHostDead(host string) bool {
	info, ok := hosts.Load(host)
	if info != nil && ok {
		return info.(*NodeAvailable).IsDead()
	}
	log.Debugf("no available info for host [%v]", host)
	return false
}

func GetHostAvailableInfo(host string) (*NodeAvailable, bool) {
	info, ok := hosts.Load(host)
	if ok && info != nil {
		return info.(*NodeAvailable), ok
	}
	return nil, false
}

var nodeAvailCache = util.NewCacheWithExpireOnAdd(1*time.Minute, 100)

func IsHostAvailable(host string) bool {
	if host == "" {
		panic("host is nil")
		return false
	}

	info, ok := GetHostAvailableInfo(host)
	if ok && info != nil {
		if global.Env().IsDebug {
			log.Trace("get host info: ", info)
		}
		if time.Since(info.lastCheck) < 60*time.Second {
			return info.IsAvailable()
		}
	}

	if global.Env().IsDebug {
		log.Tracef("no available info for host [%v]", host)
	}

	v := nodeAvailCache.Get(host)
	if v != nil {
		a, ok := v.(bool)
		if ok {
			if global.Env().IsDebug {
				log.Trace("hit cache:", host, ",", a)
			}
			return a
		}
	}

	arry := strings.Split(host, ":")
	if len(arry) == 2 {
		port, err := util.ToInt(arry[1])
		if err != nil {
			panic(err)
		}
		avail := util.TestTCPPort(arry[0], port, 10*time.Second)
		nodeAvailCache.Put(host, avail)
		if !avail {
			return false
		}
	}

	return true
}

//ip:port
/*
这是把所有有可能的地址，都获取一遍。

Config.Hosts / Config.Host  用户可能配置1个/多个host，都拿一遍

Config.Endpoint / Config.Endpoints 用户可能配置1个/多个Endpoint，都拿一遍
*/
func (meta *ElasticsearchMetadata) GetSeedHosts() []string {

	if len(meta.seedHosts) > 0 {
		return meta.seedHosts
	}

	hosts := []string{}
	if len(meta.Config.Hosts) > 0 {
		for _, h := range meta.Config.Hosts {
			hosts = append(hosts, h)
		}
	}
	if len(meta.Config.Host) > 0 {
		hosts = append(hosts, meta.Config.Host)
	}

	if meta.Config.Endpoint != "" {
		i, err := uri.Parse(meta.Config.Endpoint)
		if err != nil {
			panic(err)
		}
		hosts = append(hosts, i.Host)
	}
	if len(meta.Config.Endpoints) > 0 {
		for _, h := range meta.Config.Endpoints {
			i, err := uri.Parse(h)
			if err != nil {
				panic(err)
			}
			hosts = append(hosts, i.Host)
		}
	}
	if len(hosts) == 0 {
		panic(errors.Errorf("no valid endpoint for [%v]", meta.Config.Name))
	}
	meta.seedHosts = hosts
	return meta.seedHosts
}

func (node *NodesInfo) GetHttpPublishHost() string {
	if util.ContainStr(node.Http.PublishAddress, "/") {
		if global.Env().IsDebug {
			log.Tracef("node's public address contains `/`,try to remove prefix")
		}
		arr := strings.Split(node.Http.PublishAddress, "/")
		if len(arr) == 2 {
			return arr[1]
		}
	}
	return node.Http.PublishAddress
}

var clients = map[string]*fasthttp.Client{}
var clientLock sync.RWMutex

func (metadata *ElasticsearchMetadata) GetActivePreferredHost(host string) string {

	if host != "" {
		//get available host
		available := IsHostAvailable(host)

		if !available {
			if metadata.IsAvailable() {
				newEndpoint := metadata.GetActiveHost()
				if host != newEndpoint {
					log.Warnf("[%v] is not available, try: [%v]", host, newEndpoint)
				}
				host = newEndpoint
			}
		}
	}

	if host == "" {
		host = metadata.GetActivePreferredSeedHost()
	}

	return host
}

func (metadata *ElasticsearchMetadata) GetHttpClient(host string) *fasthttp.Client {

	if host == "" {
		panic("host can't be nil")
	}

	clientLock.RLock()
	client, ok := clients[host]
	clientLock.RUnlock()

	if !ok {
		clientLock.Lock()
		defer clientLock.Unlock()
		client = metadata.NewHttpClient(host)
		clients[host] = client
	}

	return client
}

func (metadata *ElasticsearchMetadata) NewHttpClient(host string) *fasthttp.Client {

	log.Trace("new http client: ", host)

	client := &fasthttp.Client{
		MaxConnsPerHost:               10000,
		MaxConnDuration:               0,
		MaxIdleConnDuration:           0,
		ReadTimeout:                   5 * time.Minute, // 10 minutes
		WriteTimeout:                  5 * time.Minute,
		DisableHeaderNamesNormalizing: true,
		DisablePathNormalizing:        true,
		MaxConnWaitTimeout:            0,
		TLSConfig:                     &tls.Config{InsecureSkipVerify: true},
		DialDualStack:                 true,
	}

	if metadata.Config.TrafficControl != nil && metadata.Config.TrafficControl.MaxConnectionPerNode > 0 {
		client.MaxConnsPerHost = metadata.Config.TrafficControl.MaxConnectionPerNode
	}
	return client
}

func (metadata *ElasticsearchMetadata) LastSuccess() time.Time {
	return metadata.lastSuccess
}

func (metadata *ElasticsearchMetadata) CheckNodeTrafficThrottle(node string, req, dataSize, maxWaitInMS int) {
	if metadata.Config.TrafficControl != nil && metadata.Config.TrafficControl.Enabled {

		if metadata.Config.TrafficControl.MaxWaitTimeInMs <= 0 {
			metadata.Config.TrafficControl.MaxWaitTimeInMs = 10 * 1000
		}

		if maxWaitInMS > 0 {
			metadata.Config.TrafficControl.MaxWaitTimeInMs = maxWaitInMS
		}

		maxTime := time.Duration(metadata.Config.TrafficControl.MaxWaitTimeInMs) * time.Millisecond
		startTime := time.Now()
	RetryRateLimit:

		if time.Now().Sub(startTime) < maxTime {

			if metadata.Config.TrafficControl.MaxQpsPerNode > 0 && req > 0 {
				if !rate.GetRateLimiterPerSecond(metadata.Config.ID, "req-max_qps", int(metadata.Config.TrafficControl.MaxQpsPerNode)).Allow() {
					stats.Increment(metadata.Config.ID, "req-max_qps_throttled")
					if global.Env().IsDebug {
						log.Debugf("request qps throttle on node [%v]", node)
					}
					time.Sleep(1 * time.Second)
					goto RetryRateLimit
				}
			}

			if metadata.Config.TrafficControl.MaxBytesPerNode > 0 && dataSize > 0 {
				if !rate.GetRateLimiterPerSecond(metadata.Config.ID, "req-max_bps",
					int(metadata.Config.TrafficControl.MaxBytesPerNode)).AllowN(time.Now(), dataSize) {
					stats.Increment(metadata.Config.ID, "req-max_bps_throttled")
					if global.Env().IsDebug {
						log.Debugf("request traffic throttle on node [%v]", node)
					}
					time.Sleep(1 * time.Second)
					goto RetryRateLimit
				}
			}

		} else {
			log.Warn("reached max traffic control time, throttle exit")
		}
	}
}

func (metadata *ElasticsearchMetadata) GetValue(s string) (interface{}, error) {
	if util.PrefixStr(s, "_meta.") {
		keys := strings.Split(s, ".")
		if len(keys) >= 2 {
			rootFied := keys[1]
			if rootFied != "" {
				switch rootFied {
				case "elasticsearch":
					if len(keys) > 3 {
						clusterID := keys[2]
						if clusterID != "" {
							meta := GetMetadata(clusterID)
							if meta != nil {
								if len(keys) > 3 {
									objKey := keys[3]
									switch objKey {
									case "index":
										if len(keys) > 5 {
											indexName := keys[4]
											indexOp := keys[5]
											switch indexOp {
											case "settings":
												_, indexSettings, err := meta.GetIndexSetting(indexName)
												if err == nil && len(keys) > 6 {
													keys := keys[4:]
													v, err := indexSettings.GetValue(util.JoinArray(keys, "."))
													if global.Env().IsDebug {
														log.Trace("cluster:", clusterID, "index:", indexName, ",settings key:", util.JoinArray(keys, "."), ",", v)
													}
													return v, err
												}
												break
											case "stats":
												s, err := meta.GetIndexStats(indexName)
												if err == nil && len(keys) > 6 {
													keys := keys[6:]
													v, err := s.GetValue(util.JoinArray(keys, "."))
													if global.Env().IsDebug {
														log.Trace("cluster:", clusterID, "index:", indexName, ",settings key:", util.JoinArray(keys, "."), ",", v)
													}
													return v, err
												}
												break
											}
										}
										break
									}
								}
							}
						}
					}
					break
				}
			}
		}
	}

	return nil, errors.New("key not found:" + s)
}

func (metadata *ElasticsearchMetadata) GetIndexStats(indexName string) (*util.MapStr, error) {
	if metadata.Config.MetadataCacheEnabled {
		if metadata.cache != nil {
			o, found := metadata.cache.Get("index_stats" + indexName)
			if found {
				return o.(*util.MapStr), nil
			}
		}
	}

	s, err := GetClient(metadata.Config.ID).GetIndexStats(indexName)
	if err == nil && metadata.Config.MetadataCacheEnabled {
		if metadata.cache != nil {
			metadata.cache.SetWithTTL("index_stats"+indexName, s, 1, 10*time.Second)
		}
	}
	return s, err
}

func (metadata *ElasticsearchMetadata) GetIndexSetting(index string) (string, *util.MapStr, error) {

	if metadata.Config.MetadataCacheEnabled {
		if metadata.cache != nil {
			o, found := metadata.cache.Get("index_settings" + index)
			if found {
				return index, o.(*util.MapStr), nil
			}
		}
	}

	//access local memory cache
	//access local kv store
	//access remote es API
	//if data is out of 30s, re-fetch from API

	if metadata.IndexSettings == nil {
		//fetch index settings and set cache with 30s TTL
		metadata.IndexSettings = map[string]*util.MapStr{}
		return index, nil, errors.Errorf("index [%v] setting not found", index)
	}

	indexSettings, ok := (metadata.IndexSettings)[index]
	if !ok {
		if global.Env().IsDebug {
			log.Tracef("index [%v] was not found in index settings", index)
		}

		if metadata.Aliases != nil {
			alias, ok := (*metadata.Aliases)[index]
			if ok {
				if global.Env().IsDebug {
					log.Tracef("found index [%v] in alias settings", index)
				}
				newIndex := alias.WriteIndex
				if alias.WriteIndex == "" {
					if len(alias.Index) == 1 {
						newIndex = alias.Index[0]
						if global.Env().IsDebug {
							log.Trace("found index [%v] in alias settings, no write_index, but only have one index, will use it", index)
						}
					} else {
						log.Warnf("writer_index [%v] was not found in alias [%v] settings", index, alias)
						return index, nil, errors.Error("writer_index was not found in alias settings", index, ",", alias)
					}
				}

				indexSettings, ok = (metadata.IndexSettings)[newIndex]
				if ok {
					if global.Env().IsDebug {
						log.Trace("index was found in index settings, ", index, "=>", newIndex, ",", indexSettings)
					}
					index = newIndex
					return index, indexSettings, nil

				} else {
					if global.Env().IsDebug {
						log.Tracef("writer_index [%v] was not found in index settings,", index)
					}
				}
			} else {
				if global.Env().IsDebug {
					log.Tracef("index [%v] was not found in alias settings", index)
				}
			}
		}

		if indexSettings == nil {
			//fetch single index settings
			settings, err := GetClient(metadata.Config.ID).GetIndexSettings(index)
			if err == nil && settings != nil && metadata.Config.MetadataCacheEnabled {
				//TODO set cache
				//metadata.IndexSettings[index] = settings
				if metadata.cache != nil {
					metadata.cache.SetWithTTL("index_settings"+index, settings, 1, 10*time.Second)
				}
				return index, settings, nil
			}
		}

		return index, nil, errors.Errorf("index [%v] setting not found", index)
	}
	return index, indexSettings, nil
}

func (metadata *ElasticsearchMetadata) GetIndexRoutingTable(index string) (map[string][]IndexShardRouting, error) {

	if metadata.Config.MetadataCacheEnabled {
		x, ok := metadata.cache.Get(index)
		if ok && x != nil {
			if y, ok := x.(map[string][]IndexShardRouting); ok {
				return y, nil
			}
		}
	}

	if metadata.ClusterState != nil {
		if metadata.ClusterState.RoutingTable != nil {
			table, ok := metadata.ClusterState.RoutingTable.Indices[index]
			if !ok {
				//check alias
				if global.Env().IsDebug {
					log.Tracef("index [%v] was not found in index settings,", index)
				}
				if metadata.Aliases != nil {
					alias, ok := (*metadata.Aliases)[index]
					if ok {
						if global.Env().IsDebug {
							log.Tracef("found index [%v] in alias settings,", index)
						}
						newIndex := alias.WriteIndex
						if alias.WriteIndex == "" {
							if len(alias.Index) == 1 {
								newIndex = alias.Index[0]
								if global.Env().IsDebug {
									log.Trace("found index [%v] in alias settings, no write_index, but only have one index, will use it,", index)
								}
							} else {
								//log.Warnf("writer_index [%v] was not found in alias [%v] settings,", index, alias)
								return nil, errors.Error("routing table not found and writer_index was not found in alias settings,", index, ",", alias)
							}
						}
						//try again with real index name
						return metadata.GetIndexRoutingTable(newIndex)
					} else {
						if global.Env().IsDebug {
							log.Tracef("index [%v] was not found in alias settings,", index)
						}
					}
				}
			}
			return table.Shards, nil
		}
	}

	if rate.GetRateLimiter("cluster_state_fetch", metadata.Config.ID, 1, 1, 10*time.Second).Allow() {
		log.Warnf("cluster state is nil, fetch routing table for index: %v", index)
		v, err := GetClient(metadata.Config.ID).GetIndexRoutingTable(index)
		if err == nil && v != nil && metadata.Config.MetadataCacheEnabled {
			metadata.cache.SetWithTTL(index, v, 100, 10*time.Second)
		}
		return v, err
	}

	return nil, errors.Errorf("index [%v] routing_table not found", index)

}

func (metadata *ElasticsearchMetadata) GetIndexPrimaryShardsRoutingTable(index string) ([]IndexShardRouting, error) {
	routingTable, err := metadata.GetIndexRoutingTable(index)
	if err != nil {
		return nil, err
	}

	primaryShards := []IndexShardRouting{}

	for _, shards := range routingTable {
		for _, x := range shards {
			if x.Primary {
				primaryShards = append(primaryShards, x)
			}
		}
	}

	return primaryShards, nil
}

func (metadata *ElasticsearchMetadata) GetIndexPrimaryShardRoutingTable(index string, shard int) (*IndexShardRouting, error) {
	routingTable, err := metadata.GetIndexRoutingTable(index)
	if err != nil {
		return nil, err
	}
	shards, ok := routingTable[util.ToString(shard)]
	if ok {
		for _, x := range shards {
			if x.Primary {
				return &x, nil
			}
		}
	}
	return nil, errors.New("not found")
}
