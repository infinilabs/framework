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
	"github.com/infinitbyte/framework/core/persist"
	"github.com/infinitbyte/framework/modules/elastic/orm"
)

func (module ElasticModule) Name() string {
	return "Elastic"
}

var (
	defaultConfig = PersistConfig{
		Driver: "elasticsearch",
		Elastic: &index.ElasticsearchConfig{
			Endpoint:    "http://localhost:9200",
			IndexPrefix: "app-",
		},
	}
)

func getDefaultConfig() PersistConfig {
	return defaultConfig
}

type PersistConfig struct {
	//Driver only `mysql` and `sqlite` are available
	Driver  string                     `config:"driver"`
	Elastic *index.ElasticsearchConfig `config:"elasticsearch"`
}

func (module ElasticModule) Start(cfg *Config) {

	//init config
	config := getDefaultConfig()
	cfg.Unpack(&config)

	if config.Driver == "elasticsearch" {
		client := index.ElasticsearchClient{Config: config.Elastic}
		handler := orm.ElasticORM{Client: &client}
		persist.Register(handler)
		return
	}
}

func (module ElasticModule) Stop() error {
	return nil

}

type ElasticModule struct {
}
