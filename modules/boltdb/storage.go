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

package boltdb

import (
	"github.com/infinitbyte/framework/core/config"
	"github.com/infinitbyte/framework/core/global"
	"github.com/infinitbyte/framework/core/kv"
	"github.com/infinitbyte/framework/modules/boltdb/boltdb"
	"os"
	"path"
)

var impl boltdb.BoltdbStore

func (this StorageModule) Name() string {
	return "Storage"
}

type Config struct {
}

type StorageConfig struct {
	Boltdb *Config `config:"boltdb"`
}

var (
	defaultConfig = StorageConfig{
		Boltdb: &Config{},
	}
)

func getDefaultConfig() StorageConfig {
	return defaultConfig
}

func (module StorageModule) Setup(cfg *config.Config) {

	//init config
	c := getDefaultConfig()
	cfg.Unpack(&c)

	folder := path.Join(global.Env().GetWorkingDir(), "blob")
	os.MkdirAll(folder, 0777)
	impl = boltdb.BoltdbStore{FileName: path.Join(folder, "/bolt.db")}
	err := impl.Open()
	if err != nil {
		panic(err)
	}
	kv.Register("boltdb", impl)

}

func (module StorageModule) Start() error {
	return nil
}

func (module StorageModule) Stop() error {
	return impl.Close()
}

type StorageModule struct {
}
