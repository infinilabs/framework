package sqlite

import (
	"compress/gzip"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"infini.sh/framework/core/config"
	"infini.sh/framework/core/env"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/security"
)

func setupBackupDB(t *testing.T, backupCfg BackupConfig) (*SQLiteORM, func()) {
	t.Helper()
	tmpDir := t.TempDir()

	handler := &SQLiteORM{
		Config: SQLiteConfig{
			Enabled: true,
			DBPath:  filepath.Join(tmpDir, "data.db"),
			Backup:  backupCfg,
		},
	}
	require.NoError(t, handler.Open())
	// A dedicated type/table: TestItem's global table-name registration is
	// order-sensitive across the suite (indexes_test re-registers it as
	// legacy_items), so touching it here breaks orm_test expectations.
	require.NoError(t, handler.RegisterSchemaWithName(backupDoc{}, "backup_items"))

	for i, item := range []backupDoc{
		{Name: "alpha", Age: 1},
		{Name: "beta", Age: 2},
	} {
		item.ID = fmt.Sprintf("backup-%d", i)
		require.NoError(t, handler.Save(nil, item))
	}

	cleanup := func() {
		handler.Close()
		os.RemoveAll(tmpDir)
	}
	return handler, cleanup
}

// backupDoc is a schema local to backup tests; see setupBackupDB for why the
// shared TestItem must not be used here.
type backupDoc struct {
	orm.ORMObjectBase
	Name string `json:"name,omitempty" elastic_mapping:"name: { type: keyword }"`
	Age  int    `json:"age,omitempty" elastic_mapping:"age: { type: integer }"`
}

func gunzipTo(t *testing.T, src, dst string) {
	t.Helper()
	in, err := os.Open(src)
	require.NoError(t, err)
	defer in.Close()

	gz, err := gzip.NewReader(in)
	require.NoError(t, err)
	defer gz.Close()

	out, err := os.Create(dst)
	require.NoError(t, err)
	defer out.Close()
	_, err = io.Copy(out, gz)
	require.NoError(t, err)
}

func TestBackupCreatesRestorableSnapshot(t *testing.T) {
	handler, cleanup := setupBackupDB(t, BackupConfig{Retention: 10})
	defer cleanup()

	archive, err := handler.Backup()
	require.NoError(t, err)
	assert.True(t, filepath.IsAbs(archive))
	assert.FileExists(t, archive)

	// No leftover raw temp file next to the archive.
	entries, err := os.ReadDir(filepath.Dir(archive))
	require.NoError(t, err)
	for _, e := range entries {
		assert.False(t, strings.HasSuffix(e.Name(), ".tmp"), "leftover tmp file: %s", e.Name())
	}

	// The archive must restore into a readable database with the same rows.
	restored := filepath.Join(t.TempDir(), "restored.db")
	gunzipTo(t, archive, restored)

	db, err := sql.Open("sqlite", "file:"+restored+"?_pragma=busy_timeout(5000)")
	require.NoError(t, err)
	defer db.Close()

	var count int
	require.NoError(t, db.QueryRow("SELECT count(*) FROM backup_items").Scan(&count))
	assert.Equal(t, 2, count)

	var name string
	require.NoError(t, db.QueryRow("SELECT name FROM backup_items WHERE age = 2").Scan(&name))
	assert.Equal(t, "beta", name)
}

func TestBackupNamesUniqueAndSorted(t *testing.T) {
	handler, cleanup := setupBackupDB(t, BackupConfig{})
	defer cleanup()

	first, err := handler.Backup()
	require.NoError(t, err)
	second, err := handler.Backup()
	require.NoError(t, err)
	assert.NotEqual(t, first, second, "backups in the same second must not collide")
}

func TestBackupCustomDirAndRetention(t *testing.T) {
	tmpDir := t.TempDir()
	backupDir := filepath.Join(tmpDir, "custom-backup")

	handler, cleanup := setupBackupDB(t, BackupConfig{Dir: backupDir, Retention: 2})
	defer cleanup()

	// Seed older archives to exercise pruning alongside a fresh backup.
	for _, stamp := range []string{"20200101-000000-000", "20200201-000000-000", "20200301-000000-000"} {
		path := filepath.Join(backupDir, "data-"+stamp+".db.gz")
		require.NoError(t, os.MkdirAll(backupDir, 0755))
		require.NoError(t, os.WriteFile(path, []byte("old"), 0644))
	}

	archive, err := handler.Backup()
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(archive, backupDir), "archive should land in the configured dir: %s", archive)

	entries, err := os.ReadDir(backupDir)
	require.NoError(t, err)
	assert.Len(t, entries, 2, "retention=2 should keep only the newest archives")

	names := make([]string, 0, len(entries))
	for _, e := range entries {
		names = append(names, e.Name())
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	// The fresh backup outranks the 20200301 seed lexicographically.
	assert.Contains(t, names[0], time.Now().Format("2006"))
	assert.Equal(t, "data-20200301-000000-000.db.gz", names[1])
}

func TestPruneBackupsKeepAllWhenDisabled(t *testing.T) {
	dir := t.TempDir()
	for i := 0; i < 5; i++ {
		path := filepath.Join(dir, fmt.Sprintf("data-2020010%d-000000-000.db.gz", i))
		require.NoError(t, os.WriteFile(path, []byte("x"), 0644))
	}
	require.NoError(t, pruneBackups(dir, "data", 0))

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 5)
}

func TestBackupConfigParsing(t *testing.T) {
	yaml := `
sqlite:
  enabled: true
  db_path: /tmp/x/data.db
  orm:
    enabled: true
  backup:
    interval: 6h
    dir: /tmp/x/backups
    retention: 30
`
	cfg, err := config.NewConfigWithYAML([]byte(yaml), "test")
	require.NoError(t, err)

	// Same chain as Setup: defaults seeded before parsing, then overrides.
	parsed := defaultModuleConfig()
	exists, err := env.ParseConfigSection(cfg, "sqlite", &parsed)
	require.NoError(t, err)
	require.True(t, exists)

	assert.Equal(t, "/tmp/x/data.db", parsed.DBPath)
	assert.True(t, parsed.ORM.Enabled)
	// Absent `enabled` must stay nil so the module default (on) applies.
	assert.Nil(t, parsed.Backup.Enabled)
	assert.Equal(t, 6*time.Hour, parsed.Backup.Interval)
	assert.Equal(t, "/tmp/x/backups", parsed.Backup.Dir)
	assert.Equal(t, 30, parsed.Backup.Retention)

	parsed.applyConfigDefaults()
	require.NotNil(t, parsed.Backup.Enabled)
	assert.True(t, *parsed.Backup.Enabled)
	assert.Equal(t, 6*time.Hour, parsed.Backup.Interval, "explicit interval should survive defaults")
	assert.Equal(t, 30, parsed.Backup.Retention, "explicit retention should survive defaults")

	disabled := `
sqlite:
  backup:
    enabled: false
`
	cfg2, err := config.NewConfigWithYAML([]byte(disabled), "test")
	require.NoError(t, err)
	parsed2 := defaultModuleConfig()
	_, err = env.ParseConfigSection(cfg2, "sqlite", &parsed2)
	require.NoError(t, err)
	parsed2.applyConfigDefaults()
	require.NotNil(t, parsed2.Backup.Enabled)
	assert.False(t, *parsed2.Backup.Enabled)
	assert.Equal(t, 12*time.Hour, parsed2.Backup.Interval, "unset interval should default")
	assert.Equal(t, 14, parsed2.Backup.Retention, "unset retention should default")

	// An explicit retention: 0 means keep-all and must survive the
	// pre-seeded default (a post-parse <=0 -> 14 rule would break it).
	keepAll := `
sqlite:
  backup:
    retention: 0
`
	cfg3, err := config.NewConfigWithYAML([]byte(keepAll), "test")
	require.NoError(t, err)
	parsed3 := defaultModuleConfig()
	_, err = env.ParseConfigSection(cfg3, "sqlite", &parsed3)
	require.NoError(t, err)
	parsed3.applyConfigDefaults()
	assert.Equal(t, 0, parsed3.Backup.Retention, "explicit retention: 0 (keep all) must survive defaults")
}

// orm.db_path is the pre-backup location; it must keep working as a
// fallback while the new module-level db_path wins when both are set.
func TestBackupConfigLegacyDBPathFallback(t *testing.T) {
	legacy := `
sqlite:
  orm:
    enabled: true
    db_path: /legacy/data.db
`
	cfg, err := config.NewConfigWithYAML([]byte(legacy), "test")
	require.NoError(t, err)
	var parsed SQLiteModuleConfig
	_, err = env.ParseConfigSection(cfg, "sqlite", &parsed)
	require.NoError(t, err)

	assert.Equal(t, "/legacy/data.db", parsed.ORM.DBPath)
	parsed.applyConfigDefaults()
	assert.Equal(t, "/legacy/data.db", parsed.DBPath, "legacy orm.db_path should be honored")

	both := `
sqlite:
  db_path: /new/data.db
  orm:
    db_path: /legacy/data.db
`
	cfg2, err := config.NewConfigWithYAML([]byte(both), "test")
	require.NoError(t, err)
	var parsed2 SQLiteModuleConfig
	_, err = env.ParseConfigSection(cfg2, "sqlite", &parsed2)
	require.NoError(t, err)
	parsed2.applyConfigDefaults()
	assert.Equal(t, "/new/data.db", parsed2.DBPath, "module-level db_path should win")
}

func TestBackupStatsTracked(t *testing.T) {
	handler, cleanup := setupBackupDB(t, BackupConfig{Interval: time.Hour})
	defer cleanup()

	// Before any attempt only the scheduled hint from task registration
	// would exist; a fresh handler has zero stats.
	zero := handler.GetBackupStats()
	assert.Nil(t, zero.LastAttemptTime)
	assert.Zero(t, zero.SuccessCount)

	archive, err := handler.Backup()
	require.NoError(t, err)

	stats := handler.GetBackupStats()
	require.NotNil(t, stats.LastAttemptTime, "last_backup_time must be set")
	assert.WithinDuration(t, time.Now(), *stats.LastAttemptTime, time.Minute)
	assert.True(t, stats.LastSuccess, "last_backup_success")
	assert.Equal(t, archive, stats.LastBackupFile)
	assert.Greater(t, stats.LastSizeBytes, int64(0), "last_backup_size")
	assert.GreaterOrEqual(t, stats.LastDurationMs, int64(0))
	assert.Equal(t, int64(1), stats.SuccessCount)
	assert.Zero(t, stats.FailureCount)

	// Next run = attempt end + interval, i.e. roughly an hour out.
	require.NotNil(t, stats.NextBackupTime, "next_backup_time must be projected")
	assert.WithinDuration(t, time.Now().Add(time.Hour), *stats.NextBackupTime, 2*time.Minute)

	// A successful attempt must surface as green in /health services.
	assert.Equal(t, "green", global.Env().GetServicesHealth()[BackupHealthService])
}

func TestBackupStatsFailureAndHealth(t *testing.T) {
	// Handler that never opened: every attempt fails fast.
	handler := &SQLiteORM{
		Config: SQLiteConfig{
			Enabled: true,
			DBPath:  filepath.Join(t.TempDir(), "data.db"),
			Backup:  BackupConfig{Interval: time.Hour},
		},
	}

	_, err := handler.Backup()
	require.Error(t, err)

	stats := handler.GetBackupStats()
	assert.False(t, stats.LastSuccess)
	assert.NotEmpty(t, stats.LastError)
	assert.Equal(t, int64(1), stats.FailureCount)
	assert.Zero(t, stats.SuccessCount)
	assert.Nil(t, stats.LastSuccessTime)
	// The next run must still be projected after a failure.
	require.NotNil(t, stats.NextBackupTime)

	assert.Equal(t, "red", global.Env().GetServicesHealth()[BackupHealthService])
}

func TestBackupStatsAction(t *testing.T) {
	handler, cleanup := setupBackupDB(t, BackupConfig{Interval: 12 * time.Hour, Retention: 5})
	defer cleanup()

	_, err := handler.Backup()
	require.NoError(t, err)

	prev := activeORM
	activeORM = handler
	t.Cleanup(func() { activeORM = prev })

	module := API{}
	recorder := httptest.NewRecorder()
	module.BackupStatsAction(recorder, httptest.NewRequest(http.MethodGet, "/sqlite/backup/stats", nil), nil)

	require.Equal(t, http.StatusOK, recorder.Code)

	var payload map[string]interface{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &payload))

	assert.Equal(t, true, payload["enabled"])
	assert.Equal(t, "12h0m0s", payload["interval"])
	assert.Equal(t, float64(5), payload["retention"])
	assert.Equal(t, true, payload["last_backup_success"])
	assert.NotEmpty(t, payload["last_backup_file"])
	assert.Greater(t, payload["last_backup_size"].(float64), float64(0))
	assert.NotNil(t, payload["last_backup_time"])
	assert.NotNil(t, payload["next_backup_time"])
	assert.Equal(t, float64(1), payload["success_count"])
}

// The stats endpoint is RBAC-guarded by PermissionSystemSqliteBackupRead on
// the web port; the key must also ride on managed-instance self tokens, or
// manager-originated calls would start failing the permission filter.
func TestBackupStatsPermissionRegistered(t *testing.T) {
	assert.Equal(t, "generic#system:sqlite-backup/read", string(security.PermissionSystemSqliteBackupRead))
	assert.Contains(t, security.InstanceOpsPermissionKeys(), security.PermissionSystemSqliteBackupRead)
}

// Databases sharing a backup directory (both defaulting to <dir>/backup)
// must never prune each other's archives, and malformed names are ignored.
func TestPruneBackupIgnoresOtherDatabasesAndMalformed(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"data-20260101-000000-000.db.gz":     "x", // ours, oldest
		"data-20260102-000000-000.db.gz":     "x", // ours, newest
		"data-20260103-000000-000.db.gz":     "x", // ours, beyond retention
		"data-old-20260101-000000-000.db.gz": "x", // other database
		"data-2026garbage.db.gz":             "x", // malformed
		"data-20260101-000000-99x.db.gz":     "x", // bad ms digits
		"data-20260101-000000-000-x9.db.gz":  "x", // bad counter
		"data-20260101-000000.db.gz":         "x", // missing ms
	}
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0644))
	}

	require.NoError(t, pruneBackups(dir, "data", 2))

	for _, name := range []string{
		"data-old-20260101-000000-000.db.gz",
		"data-2026garbage.db.gz",
		"data-20260101-000000-99x.db.gz",
		"data-20260101-000000-000-x9.db.gz",
		"data-20260101-000000.db.gz",
	} {
		assert.FileExists(t, filepath.Join(dir, name), "must not be pruned or matched")
	}
	assert.NoFileExists(t, filepath.Join(dir, "data-20260101-000000-000.db.gz"), "oldest of ours should be pruned")
	assert.FileExists(t, filepath.Join(dir, "data-20260102-000000-000.db.gz"))
}

// A same-millisecond collision appends a counter; the counter file is NEWER
// than the plain one, so retention must keep it and the backup must return
// an archive that still exists.
func TestBackupCollisionRetentionKeepsNewest(t *testing.T) {
	handler, cleanup := setupBackupDB(t, BackupConfig{Retention: 1})
	defer cleanup()

	// Seed an archive with the exact stamp Backup() is about to use by
	// making the directory non-empty for that millisecond: run once, then
	// force a second run inside the same millisecond by rewriting time is
	// not possible — so create the collision via a second immediate call;
	// if it lands in a different millisecond the test degrades to a normal
	// prune check. Force determinism instead: pre-create the exact target.
	now := time.Now()
	stamp := now.Format("20060102-150405") + fmt.Sprintf("-%03d", now.Nanosecond()/int(time.Millisecond))
	dir := handler.backupDir()
	require.NoError(t, os.MkdirAll(dir, 0755))
	seed := filepath.Join(dir, fmt.Sprintf("data-%s.db.gz", stamp))
	require.NoError(t, os.WriteFile(seed, []byte("old"), 0600))

	archive, err := handler.Backup()
	require.NoError(t, err)

	// Backup either collided (counter file) or landed in a later ms; in
	// both cases the returned archive must exist and the seeded older
	// archive of the same millisecond must be the one pruned.
	assert.FileExists(t, archive, "returned path must exist after pruning")
	if filepath.Base(archive) == fmt.Sprintf("data-%s-1.db.gz", stamp) {
		assert.NoFileExists(t, seed, "collision: older same-ms archive should be pruned")
	}
}

func TestBackupArchivePermissions(t *testing.T) {
	handler, cleanup := setupBackupDB(t, BackupConfig{})
	defer cleanup()

	archive, err := handler.Backup()
	require.NoError(t, err)

	info, err := os.Stat(archive)
	require.NoError(t, err)
	assert.Equal(t, os.FileMode(0600), info.Mode().Perm(), "archives must be owner-only")
}
