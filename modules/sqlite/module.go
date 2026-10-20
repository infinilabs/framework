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

	// backupStop signals the backup worker to exit; backupDone is closed by
	// the worker once the in-flight backup (if any) has finished.
	backupStop chan struct{}
	backupDone chan struct{}
}

var moduleConfig = defaultModuleConfig()

func (module *SQLiteModule) Name() string {
	return "sqlite"
}

func (module *SQLiteModule) Setup() {
	// Start from defaults so an explicit `retention: 0` (keep all) survives
	// parsing instead of being re-defaulted to 14.
	moduleConfig = defaultModuleConfig()
	exists, err := env.ParseConfig("sqlite", &moduleConfig)
	if exists && err != nil && global.Env().SystemConfig.Configs.PanicOnConfigError {
		panic(err)
	}

	moduleConfig.applyConfigDefaults()

	// Default DB path
	if moduleConfig.DBPath == "" {
		moduleConfig.DBPath = filepath.Join(global.Env().GetDataDir(), "sqlite", "data.db")
	}
}

func (module *SQLiteModule) Start() error {
	if !moduleConfig.Enabled {
		log.Debug("sqlite module is not enabled, skipping")
		return nil
	}

	if moduleConfig.ORM.Enabled {
		handler := &SQLiteORM{Config: SQLiteConfig{
			Enabled: true,
			DBPath:  moduleConfig.DBPath,
			Backup:  moduleConfig.Backup,
		}}
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

		// Periodic compressed backup. First run shortly after start so a
		// restart always yields a fresh baseline snapshot, then every
		// Backup.Interval (default 12h).
		//
		// The backup runs on a dedicated worker instead of the shared
		// background-callback loop: that loop executes tasks serially and
		// cannot interrupt one, so a long VACUUM+gzip would stall unrelated
		// callbacks (config polling, PRAGMA optimize) for the whole backup.
		if moduleConfig.Backup.Enabled != nil && *moduleConfig.Backup.Enabled && moduleConfig.Backup.Interval > 0 {
			interval := moduleConfig.Backup.Interval
			initialDelay := 1 * time.Minute
			module.backupStop = make(chan struct{})
			module.backupDone = make(chan struct{})
			stop, done := module.backupStop, module.backupDone
			go func() {
				defer close(done)
				select {
				case <-time.After(initialDelay):
				case <-stop:
					return
				}
				for {
					if _, err := handler.Backup(); err != nil {
						log.Errorf("sqlite periodic backup: %v", err)
					}
					select {
					case <-time.After(interval):
					case <-stop:
						return
					}
				}
			}()
			// Show when the first backup is due before it has ever run;
			// every attempt overwrites this in recordBackupResult.
			handler.markBackupScheduled(initialDelay)
			activeORM = handler
			log.Infof("sqlite periodic backup enabled, interval: %v, dir: %s, retention: %d",
				interval, moduleConfig.Backup.Dir, moduleConfig.Backup.Retention)
		}
	}

	return nil
}

func (module *SQLiteModule) Stop() error {
	// Signal the backup worker and wait out any in-flight backup before
	// closing the database underneath it.
	if module.backupStop != nil {
		close(module.backupStop)
		<-module.backupDone
	}
	if module.ormHandler != nil {
		return module.ormHandler.Close()
	}
	return nil
}
