// Copyright (C) INFINI Labs & INFINI LIMITED.
//
// The INFINI Framework is offered under the GNU Affero General Public License v3.0
// and as commercial software.
//
// For commercial licensing, contact us at:
//   - Website: infinilabs.com
//   - Email: hello@infini.ltd
//
// Open Source licensed under AGPL V3:
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package sqlite

import (
	"path/filepath"
	"time"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/orm"
)

// SQLiteModule implements the framework module interface for the SQLite ORM backend.
type SQLiteModule struct {
	ormHandler *SQLiteORM
}

var moduleConfig = SQLiteModuleConfig{}

func (module *SQLiteModule) Name() string {
	return "sqlite"
}

func (module *SQLiteModule) Setup() {
	exists, err := env.ParseConfig("sqlite", &moduleConfig)
	if exists && err != nil && global.Env().SystemConfig.Configs.PanicOnConfigError {
		panic(err)
	}

	// Default DB path
	if moduleConfig.ORM.DBPath == "" {
		moduleConfig.ORM.DBPath = filepath.Join(global.Env().GetDataDir(), "sqlite", "data.db")
	}
}

func (module *SQLiteModule) Start() error {
	if !moduleConfig.Enabled {
		log.Debug("sqlite module is not enabled, skipping")
		return nil
	}

	if moduleConfig.ORM.Enabled {
		handler := &SQLiteORM{Config: moduleConfig.ORM}
		if err := handler.Open(); err != nil {
			return err
		}
		module.ormHandler = handler
		orm.Register("sqlite", handler)
		log.Info("sqlite ORM backend registered")

		// Init schemas
		if err := orm.InitSchema(); err != nil {
			return err
		}

		// Keep planner statistics fresh. Registration optimizes EMPTY
		// tables; data arrives at runtime and without sqlite_stat1 the
		// planner over/under-estimates and may skip covering composites
		// (measured: 1.18s vs 0.76s per dashboard aggregation at 500k rows).
		// PRAGMA optimize is near-free when nothing changed — the SQLite
		// recommended cadence is "every few hours and after bulk changes".
		task := global.BackgroundTask{}
		task.Tag = "sqlite_optimize"
		task.Func = func() {
			if _, err := handler.DB.Exec("PRAGMA optimize"); err != nil {
				log.Warnf("sqlite periodic PRAGMA optimize: %v", err)
			}
		}
		task.Interval = 10 * time.Minute
		task.InitialDelay = 10 * time.Minute

		// statsLoop refreshes planner statistics periodically.
		global.RegisterBackgroundCallback(&task)
	}

	return nil
}

func (module *SQLiteModule) Stop() error {
	if module.ormHandler != nil {
		return module.ormHandler.Close()
	}
	return nil
}
