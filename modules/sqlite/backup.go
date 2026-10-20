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
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
)

// backupTimeLayout zero-pads to milliseconds so names stay unique within one
// second and still sort lexicographically in chronological order.
const backupTimeLayout = "20060102-150405-000"

// BackupHealthService is the name this module reports under in
// /health services: green after a successful backup, red after a failed one.
const BackupHealthService = "sqlite_backup"

// backupMu serializes backups; VACUUM INTO is online-safe, but two concurrent
// runs would burn I/O for identical snapshots and race on pruning.
var backupMu sync.Mutex

// quoteSQLString renders path as a SQL string literal (” escapes ').
func quoteSQLString(path string) string {
	return "'" + strings.ReplaceAll(path, "'", "''") + "'"
}

// BackupStats is a point-in-time snapshot of backup activity, exposed via
// /sqlite/backup/stats and intended to answer: when did the last backup run,
// how long did it take, how big is it, did it succeed, and when is the next
// one due.
type BackupStats struct {
	LastAttemptTime *time.Time `json:"last_backup_time"`
	LastSuccessTime *time.Time `json:"last_success_time"`
	LastSuccess     bool       `json:"last_backup_success"`
	LastError       string     `json:"last_backup_error,omitempty"`
	LastDurationMs  int64      `json:"last_backup_duration_ms"`
	LastSizeBytes   int64      `json:"last_backup_size"`
	LastBackupFile  string     `json:"last_backup_file,omitempty"`
	NextBackupTime  *time.Time `json:"next_backup_time"`
	SuccessCount    int64      `json:"success_count"`
	FailureCount    int64      `json:"failure_count"`
}

// GetBackupStats returns a copy of the current backup statistics.
func (handler *SQLiteORM) GetBackupStats() BackupStats {
	handler.statsMu.RLock()
	defer handler.statsMu.RUnlock()
	return handler.backupStats
}

// markBackupScheduled seeds the next-run hint shown in stats before the first
// backup has fired; the background task overwrites it after every attempt.
func (handler *SQLiteORM) markBackupScheduled(delay time.Duration) {
	t := time.Now().Add(delay)
	handler.statsMu.Lock()
	handler.backupStats.NextBackupTime = &t
	handler.statsMu.Unlock()
}

// backupDir resolves the archive directory: configured Dir or <db dir>/backup.
func (handler *SQLiteORM) backupDir() string {
	if handler.Config.Backup.Dir != "" {
		return handler.Config.Backup.Dir
	}
	return filepath.Join(filepath.Dir(handler.Config.DBPath), "backup")
}

// recordBackupResult updates stats after an attempt and recomputes when the
// scheduler is expected to fire next (attempt end + interval).
func (handler *SQLiteORM) recordBackupResult(start time.Time, path string, size int64, err error) {
	end := time.Now()
	handler.statsMu.Lock()
	s := &handler.backupStats
	s.LastAttemptTime = &start
	s.LastDurationMs = end.Sub(start).Milliseconds()
	if err != nil {
		s.LastSuccess = false
		s.LastError = err.Error()
		s.FailureCount++
	} else {
		s.LastSuccess = true
		s.LastError = ""
		s.SuccessCount++
		s.LastBackupFile = path
		s.LastSizeBytes = size
		s.LastSuccessTime = &end
	}
	if interval := handler.Config.Backup.Interval; interval > 0 {
		next := end.Add(interval)
		s.NextBackupTime = &next
	}
	handler.statsMu.Unlock()
}

// Backup writes a gzip-compressed, consistent snapshot of the database to
// the backup dir and prunes archives beyond Backup.Retention. It is safe to
// call while the database is serving reads and writes. Every attempt updates
// the backup stats and the sqlite_backup service health.
func (handler *SQLiteORM) Backup() (string, error) {
	backupMu.Lock()
	defer backupMu.Unlock()

	start := time.Now()
	path, size, err := handler.snapshot()
	handler.recordBackupResult(start, path, size, err)

	if err != nil {
		global.Env().ReportHealth(BackupHealthService, env.HEALTH_RED)
	} else {
		global.Env().ReportHealth(BackupHealthService, env.HEALTH_GREEN)
	}
	return path, err
}

// snapshot performs the actual backup; the caller holds backupMu.
func (handler *SQLiteORM) snapshot() (string, int64, error) {
	if handler.DB == nil {
		return "", 0, fmt.Errorf("sqlite backup: database is not open")
	}

	dir := handler.backupDir()
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", 0, fmt.Errorf("sqlite backup: create backup dir %s: %w", dir, err)
	}

	stamp := time.Now().Format(backupTimeLayout)
	base := strings.TrimSuffix(filepath.Base(handler.Config.DBPath), filepath.Ext(handler.Config.DBPath))
	finalPath := filepath.Join(dir, fmt.Sprintf("%s-%s.db.gz", base, stamp))
	// Two backups inside one millisecond (manual runs, tests) bump a counter;
	// the "-N" suffix still sorts right after the original timestamp.
	for i := 1; util.FileExists(finalPath); i++ {
		finalPath = filepath.Join(dir, fmt.Sprintf("%s-%s-%d.db.gz", base, stamp, i))
	}
	tmpPath := strings.TrimSuffix(finalPath, ".gz") + ".tmp"

	// VACUUM INTO refuses to overwrite an existing file, so a leftover tmp
	// from a crashed run must go first.
	os.Remove(tmpPath)

	// VACUUM INTO produces a defragmented copy of the database including all
	// committed WAL content — no separate checkpoint needed.
	if _, err := handler.DB.Exec("VACUUM INTO " + quoteSQLString(tmpPath)); err != nil {
		os.Remove(tmpPath)
		return "", 0, fmt.Errorf("sqlite backup: VACUUM INTO: %w", err)
	}
	defer os.Remove(tmpPath)

	if err := gzipFile(tmpPath, finalPath); err != nil {
		os.Remove(finalPath)
		return "", 0, fmt.Errorf("sqlite backup: compress: %w", err)
	}

	if err := pruneBackups(dir, base, handler.Config.Backup.Retention); err != nil {
		log.Warnf("sqlite backup: prune old backups: %v", err)
	}

	var size int64
	if info, err := os.Stat(finalPath); err == nil {
		size = info.Size()
		log.Infof("sqlite backup completed: %s (%s)", finalPath, util.ByteSize(uint64(size)))
	} else {
		log.Infof("sqlite backup completed: %s", finalPath)
	}
	return finalPath, size, nil
}

// gzipFile streams src into dst.gz-level compression without holding the
// database in memory.
func gzipFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}

	gw := gzip.NewWriter(out)
	if _, err := io.Copy(gw, in); err != nil {
		gw.Close()
		out.Close()
		return err
	}
	if err := gw.Close(); err != nil {
		out.Close()
		return err
	}
	return out.Close()
}

// pruneBackups keeps the newest retention archives named <base>-*.db.gz and
// deletes the rest. retention <= 0 keeps everything.
func pruneBackups(dir, base string, retention int) error {
	if retention <= 0 {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	var archives []string
	pattern := base + "-"
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() && strings.HasPrefix(name, pattern) && strings.HasSuffix(name, ".db.gz") {
			archives = append(archives, name)
		}
	}
	// Names embed backupTimeLayout, so lexicographic == chronological.
	sort.Sort(sort.Reverse(sort.StringSlice(archives)))

	if retention >= len(archives) {
		return nil
	}
	for _, name := range archives[retention:] {
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			log.Warnf("sqlite backup: remove old archive %s: %v", name, err)
		} else {
			log.Debugf("sqlite backup: pruned old archive %s", name)
		}
	}
	return nil
}
