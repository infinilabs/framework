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
	"strconv"
	"strings"
	"sync"
	"time"

	log "github.com/cihub/seelog"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
)

// backupStampLayout is the lex-sortable timestamp part of archive names.
// The millisecond part is appended separately with %03d: Go layouts express
// fractional seconds only as ".000" (dot-prefixed), so "-000" inside a
// layout would be literal text and every name would carry a constant 000.
const backupStampLayout = "20060102-150405"

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
	handler.backupInFlight.Add(1)
	defer handler.backupInFlight.Done()

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

	now := time.Now()
	stamp := now.Format(backupStampLayout) + fmt.Sprintf("-%03d", now.Nanosecond()/int(time.Millisecond))
	base := strings.TrimSuffix(filepath.Base(handler.Config.DBPath), filepath.Ext(handler.Config.DBPath))
	finalPath := filepath.Join(dir, fmt.Sprintf("%s-%s.db.gz", base, stamp))
	// Two backups inside one millisecond (manual runs, tests) bump a counter;
	// pruneBackups orders by parsed timestamp+counter, so the counter suffix
	// must follow the format validated by parseArchiveName.
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
	// SQLite creates the snapshot umask-dependent (0644 under 022); the raw
	// database copy must not be more readable than the live file.
	if err := os.Chmod(tmpPath, 0600); err != nil {
		log.Warnf("sqlite backup: chmod %s: %v", tmpPath, err)
	}

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

	// 0600, not os.Create's 0666: archives are full copies of the database
	// and must not widen access to it for other local users.
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
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

// parseArchiveName reports whether name is an archive of the database base
// with the exact form <base>-<YYYYMMDD-HHMMSS>-<mmm>[-<n>].db.gz, returning
// its millisecond-resolution timestamp key and sequence counter. A plain
// prefix test is not enough: with /dir/data.db and /dir/data-old.db both
// defaulting to /dir/backup, pruning for "data" must not touch
// data-old-*.db.gz, and unrelated or malformed files must be ignored.
func parseArchiveName(name, base string) (key string, counter int, ok bool) {
	rest := strings.TrimPrefix(name, base+"-")
	if len(rest) == len(name) || !strings.HasSuffix(rest, ".db.gz") {
		return "", 0, false
	}
	rest = strings.TrimSuffix(rest, ".db.gz")

	// rest = <YYYYMMDD>-<HHMMSS>-<mmm>[-<n>]; the stamp itself contains a
	// dash, so split fields are date, time, millis, optional counter.
	parts := strings.Split(rest, "-")
	if len(parts) != 3 && len(parts) != 4 {
		return "", 0, false
	}
	if _, err := time.Parse(backupStampLayout, parts[0]+"-"+parts[1]); err != nil {
		return "", 0, false
	}
	// milliseconds: exactly three digits
	if len(parts[2]) != 3 || strings.Trim(parts[2], "0123456789") != "" {
		return "", 0, false
	}
	if len(parts) == 4 {
		// same-millisecond collision counter: non-empty digits, >= 1
		if parts[3] == "" || strings.Trim(parts[3], "0123456789") != "" {
			return "", 0, false
		}
		c, err := strconv.Atoi(parts[3])
		if err != nil || c < 1 {
			return "", 0, false
		}
		return parts[0] + parts[1] + parts[2], c, true
	}
	return parts[0] + parts[1] + parts[2], 0, true
}

// pruneBackups keeps the newest retention archives of this database and
// deletes the rest, newest-first by timestamp then collision counter.
// retention <= 0 keeps everything.
func pruneBackups(dir, base string, retention int) error {
	if retention <= 0 {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	type archiveRef struct {
		key     string // YYYYMMDD-HHMMSSmmm, fixed width, lex == chronological
		counter int
		name    string
	}
	var archives []archiveRef
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if key, counter, ok := parseArchiveName(e.Name(), base); ok {
			archives = append(archives, archiveRef{key: key, counter: counter, name: e.Name()})
		}
	}
	// Sort newest-first by timestamp, then by collision counter: for the same
	// millisecond, data-...-000-1.db.gz was created AFTER data-...-000.db.gz
	// and must be kept ahead of it under retention.
	sort.Slice(archives, func(i, j int) bool {
		if archives[i].key != archives[j].key {
			return archives[i].key > archives[j].key
		}
		return archives[i].counter > archives[j].counter
	})

	if retention >= len(archives) {
		return nil
	}
	for _, a := range archives[retention:] {
		if err := os.Remove(filepath.Join(dir, a.name)); err != nil {
			log.Warnf("sqlite backup: remove old archive %s: %v", a.name, err)
		} else {
			log.Debugf("sqlite backup: pruned old archive %s", a.name)
		}
	}
	return nil
}
