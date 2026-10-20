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

import "time"

// SQLiteConfig holds configuration for the SQLite ORM backend.
type SQLiteConfig struct {
	Enabled bool         `config:"enabled"`
	DBPath  string       `config:"db_path"`
	Backup  BackupConfig `config:"backup"`
}

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
}

// SQLiteModuleConfig is the top-level module configuration.
type SQLiteModuleConfig struct {
	Enabled bool         `config:"enabled"`
	ORM     SQLiteConfig `config:"orm"`
}
