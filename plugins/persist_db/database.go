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

package persist_db

import (
	. "infini.sh/framework/core/config"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/plugins/persist_db/mysql"
	"infini.sh/framework/plugins/persist_db/sqlite"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	_ "github.com/jinzhu/gorm/dialects/sqlite"
)

func (module DatabaseModule) Name() string {
	return "database"
}

var (
	defaultConfig = PersistConfig{
		Driver: "sqlite",
		SQLite: &sqlite.SQLiteConfig{},
		MySQL:  &mysql.MySQLConfig{},
	}
)

type PersistConfig struct {
	//Driver only `mysql` and `sqlite` are available
	Driver string               `config:"driver"`
	SQLite *sqlite.SQLiteConfig `config:"sqlite"`
	MySQL  *mysql.MySQLConfig   `config:"mysql"`
}

func (module DatabaseModule) Setup(cfg *Config) {
	//init config
	cfg.Unpack(&defaultConfig)
}

func (module DatabaseModule) Start() error {

	//whether use lock, only sqlite need lock
	userLock := false
	if defaultConfig.Driver == "sqlite" {
		db = sqlite.GetInstance(defaultConfig.SQLite)
		userLock = true
	} else if defaultConfig.Driver == "mysql" {
		db = mysql.GetInstance(defaultConfig.MySQL)
	} else {
		panic(errors.Errorf("invalid driver, %s", defaultConfig.Driver))
	}

	handler := SQLORM{conn: db, useLock: userLock}

	orm.Register("db", handler)

	return nil
}

func (module DatabaseModule) Stop() error {
	if db != nil {
		db.Close()
	}
	return nil

}

type DatabaseModule struct {
}
