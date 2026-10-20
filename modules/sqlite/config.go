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
	"time"

	log "github.com/cihub/seelog"
)

// BackupConfig controls periodic online backup of the SQLite database.
//
// The backup uses `VACUUM INTO`, which writes a compact, always-consistent
// snapshot without blocking readers, then gzips it in place. Old archives
// beyond Retention are pruned after each run.
type BackupConfig struct {
	// Enabled defaults to true; the pointer distinguishes "not configured"
	// (nil -> default on) from an explicit `enabled: false`.
	Enabled   *bool         `config:"enabled"`
	Interval  time.Duration `config:"interval"`  // default 12h
	Dir       string        `config:"dir"`       // default: <db dir>/backup
	Retention int           `config:"retention"` // max archives to keep, default 14
	// HighLoadThreshold is the total CPU usage percent above which the
	// scheduled backup skips itself with a warning (default 90). A backup
	// can still be forced via POST /sqlite/backup/_run; 0 disables the gate.
	HighLoadThreshold float64 `config:"high_load_threshold"`
}

// SQLiteConfig is the runtime configuration of a SQLiteORM handler; the
// module assembles it from the parsed YAML sections (see SQLiteModuleConfig).
type SQLiteConfig struct {
	Enabled bool
	DBPath  string
	Backup  BackupConfig
}

// ORMSectionConfig is the `sqlite.orm` section, which only controls whether
// the SQLite backend is registered with the ORM registry. DBPath here is
// the deprecated pre-backup location, honored as a fallback so existing
// configurations keep working.
type ORMSectionConfig struct {
	Enabled bool   `config:"enabled"`
	DBPath  string `config:"db_path"` // deprecated: use sqlite.db_path
}

// SQLiteModuleConfig is the top-level `sqlite` module configuration. The
// database file (db_path) and its operations (backup) are module-level
// concerns and sit beside `orm`, not under it.
type SQLiteModuleConfig struct {
	Enabled bool             `config:"enabled"`
	DBPath  string           `config:"db_path"`
	Backup  BackupConfig     `config:"backup"`
	ORM     ORMSectionConfig `config:"orm"`
}

// defaultModuleConfig returns the pre-parse defaults. Retention is seeded
// BEFORE parsing rather than defaulted after: an explicit `retention: 0`
// means keep-all (see BackupConfig), and a post-parse `<= 0 -> 14` rule
// would silently override it.
func defaultModuleConfig() SQLiteModuleConfig {
	c := SQLiteModuleConfig{}
	c.Backup.Retention = 14
	c.Backup.HighLoadThreshold = 90
	return c
}

// applyConfigDefaults resolves the deprecated orm.db_path fallback and
// applies post-parse backup defaults; Setup adds the data-dir default for
// DBPath.
func (c *SQLiteModuleConfig) applyConfigDefaults() {
	if c.DBPath == "" && c.ORM.DBPath != "" {
		log.Warn("sqlite: orm.db_path is deprecated, use sqlite.db_path")
		c.DBPath = c.ORM.DBPath
	}
	if c.Backup.Enabled == nil {
		enabled := true
		c.Backup.Enabled = &enabled
	}
	if c.Backup.Interval <= 0 {
		c.Backup.Interval = 12 * time.Hour
	}
}
