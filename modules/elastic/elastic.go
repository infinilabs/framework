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

package elastic

import (
	. "github.com/infinitbyte/framework/core/config"
	"github.com/infinitbyte/framework/core/index"
	"github.com/infinitbyte/framework/core/kv"
	"github.com/infinitbyte/framework/core/orm"
	o "github.com/infinitbyte/framework/modules/elastic/orm"
	"github.com/infinitbyte/framework/modules/elastic/store"
)

func (module ElasticModule) Name() string {
	return "Elastic"
}

var (
	defaultConfig = ElasticConfig{
		Elastic: &index.ElasticsearchConfig{
			Endpoint:    "http://localhost:9200",
			IndexPrefix: "app-",
		},
	}
)

func getDefaultConfig() ElasticConfig {
	return defaultConfig
}

type ElasticConfig struct {
	KVEnabled    bool                       `config:"kv_enabled"`
	ORMEnabled   bool                       `config:"orm_enabled"`
	IndexEnabled bool                       `config:"index_enabled"`
	Elastic      *index.ElasticsearchConfig `config:"elasticsearch"`
}

func (module ElasticModule) Setup(cfg *Config) {

	//init config
	config := getDefaultConfig()
	cfg.Unpack(&config)

	client := index.ElasticsearchClient{Config: config.Elastic}
	if config.ORMEnabled {
		handler := o.ElasticORM{Client: &client}
		orm.Register("elastic", handler)
	}

	if config.KVEnabled {
		storeHandler := store.ElasticsearchStore{Client: &client}
		kv.Register("elastic", storeHandler)
	}

}

func (module ElasticModule) Stop() error {
	return nil

}

func (module ElasticModule) Start() error {
	return nil

}

type ElasticModule struct {
}
