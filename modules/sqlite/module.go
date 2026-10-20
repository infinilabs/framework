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
	"context"
	"path/filepath"
	"time"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/task"
)

// SQLiteModule implements the framework module interface for the SQLite ORM backend.
type SQLiteModule struct {
	ormHandler   *SQLiteORM
	backupTaskID string
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
		optimizeTask := global.BackgroundTask{}
		optimizeTask.Tag = "sqlite_optimize"
		optimizeTask.Func = func() {
			if _, err := handler.DB.Exec("PRAGMA optimize"); err != nil {
				log.Warnf("sqlite periodic PRAGMA optimize: %v", err)
			}
		}
		optimizeTask.Interval = 10 * time.Minute
		optimizeTask.InitialDelay = 10 * time.Minute

		// statsLoop refreshes planner statistics periodically.
		global.RegisterBackgroundCallback(&optimizeTask)

		// Periodic compressed backup, registered with the framework task
		// scheduler rather than the shared background-callback loop: chrono
		// runs every execution on its own goroutine (a long VACUUM+gzip
		// cannot stall config polling or PRAGMA optimize), Singleton skips
		// overlapping runs, the task is visible in the task registry, and
		// TaskModule.Stop coordinates shutdown. The first run is due one
		// full Backup.Interval (default 12h) after startup, then every
		// interval; note a restart resets that schedule, and POST
		// /sqlite/backup/_run covers the gap when needed.
		if moduleConfig.Backup.Enabled != nil && *moduleConfig.Backup.Enabled && moduleConfig.Backup.Interval > 0 {
			interval := moduleConfig.Backup.Interval
			loadThreshold := moduleConfig.Backup.HighLoadThreshold
			module.backupTaskID = task.RegisterScheduleTask(task.ScheduleTask{
				ID:          "sqlite_backup",
				Group:       "sqlite",
				Description: "compressed online backup of the sqlite database",
				Type:        task.Interval,
				Interval:    interval.String(),
				// The first backup is due one full interval after startup -
				// a backup at boot adds I/O right where the process is
				// warming up and duplicates whatever the previous run
				// archived; forced runs via the API cover the gap.
				InitialDelay: interval.String(),
				Singleton:    true,
				Task: func(ctx context.Context) {
					// A busy node skips its backup with a warning instead of
					// adding VACUUM+gzip I/O on top of the load; operators
					// can still force a run via the API.
					if handler.SkipAutoBackupUnderLoad(loadThreshold) {
						return
					}
					if _, err := handler.Backup(); err != nil {
						log.Errorf("sqlite periodic backup: %v", err)
					}
				},
			})
			// Show when the next backup is due before the first attempt has
			// run; every attempt overwrites this in recordBackupResult.
			handler.markBackupScheduled(interval)
			activeORM = handler
			log.Infof("sqlite periodic backup enabled, task: %s, interval: %v, dir: %s, retention: %d",
				module.backupTaskID, interval, moduleConfig.Backup.Dir, moduleConfig.Backup.Retention)
		}
	}

	return nil
}

func (module *SQLiteModule) Stop() error {
	// Stop future backup runs, then wait out any in-flight backup before
	// closing the database underneath it. Module stop order is not
	// guaranteed, so this also covers the case where TaskModule (and its
	// scheduler shutdown) has not stopped yet.
	if module.backupTaskID != "" {
		task.StopTask(module.backupTaskID)
	}
	if module.ormHandler != nil {
		module.ormHandler.waitBackup()
		return module.ormHandler.Close()
	}
	return nil
}
