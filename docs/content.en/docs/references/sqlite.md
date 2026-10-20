---
title: "SQLite Storage"
weight: 51
---
# SQLite Storage

The framework ships an embedded SQLite ORM backend (see [ORM](orm/)) for applications that need a zero-dependency local metadata store. The `sqlite` module runs the database in WAL mode and adds operational safety nets on top: periodic online backup with gzip compression, retention-based cleanup, backup statistics, and health reporting.

## Module Configuration

The database file and its operations are module-level concerns and sit beside `orm`, which only controls whether the SQLite backend is registered with the ORM registry:

```yaml
sqlite:
  enabled: true
  db_path: data/sqlite/data.db   # default: <data_dir>/sqlite/data.db
  orm:
    enabled: true
  backup:
    enabled: true                # default: true; set false to disable backups
    interval: 12h                # default: 12h
    dir: ""                      # default: <db_path dir>/backup
    retention: 14                # max archives to keep, default: 14
    high_load_threshold: 90      # skip scheduled backup above this CPU %, default: 90; 0 disables the gate
    first_backup_delay: 0s       # first backup after startup; default: one full interval
```

| Field              | Description                                                              |
|--------------------|--------------------------------------------------------------------------|
| `db_path`          | Database file path. Defaults to `<data_dir>/sqlite/data.db`. The deprecated `orm.db_path` location is still honored as a fallback. |
| `orm.enabled`      | Whether the SQLite backend is registered with the ORM registry.          |
| `backup.enabled`   | Whether periodic backup runs. Defaults to `true`; explicit `false` opts out. |
| `backup.interval`  | Time between backups (Go duration string, e.g. `30m`, `12h`). Default `12h`. |
| `backup.dir`       | Directory for backup archives. Defaults to a `backup` folder next to the database file. |
| `backup.retention` | Maximum number of archives kept; older ones are pruned after each run. Default `14`. Set `0` to keep everything. |
| `backup.high_load_threshold` | Total CPU usage percent above which a scheduled backup skips itself with a warning instead of adding backup I/O to a busy node. Default `90`; `0` disables the check. Forced runs via the API always proceed. |
| `backup.first_backup_delay` | When the first backup after startup is due. Unset or zero means one full `interval` (default behavior — a boot-time backup only adds I/O during warm-up and duplicates the previous archive). Set a positive duration (e.g. `1m`) for an early baseline backup after restarts. |

## How Backup Works

The backup is registered with the framework task scheduler (visible in the task registry, e.g. `GET /tasks`, under the ID `sqlite_backup`); each execution runs on its own goroutine, so a long backup never stalls other scheduled tasks, and overlapping runs are skipped. The first backup is due one full `interval` after startup — a backup at boot would only add I/O where the process is warming up and duplicate whatever the previous run already archived — then every `interval`. Set `first_backup_delay` (e.g. `1m`) to opt into an early baseline backup instead. Note that a restart resets this schedule (a process restarted more often than `interval` would never back up automatically); cover the gap with a forced `POST /sqlite/backup/_run`.

> **Note**: the scheduling rides on the framework's `task` module. Applications that enable the SQLite ORM must also register `task.TaskModule` (as gateway, logpilot, ops, and transfer all do); without it the backup stays pending and never runs.

Each backup uses SQLite's `VACUUM INTO`, which writes a compact, always-consistent snapshot of the database (including all committed WAL content) without blocking readers or writers. The snapshot is then gzip-compressed in place; the database is never held in memory as a whole.

Archives are named `<db_name>-<timestamp>.db.gz` with millisecond resolution, e.g.:

```
data-20261016-150405-123.db.gz
```

Archives are written with owner-only permissions (`0600`): they are full copies of the database and must not widen access to it for other local users, even when the backup directory itself is traversable. Pruning matches only this database's exact archive naming, so databases sharing a backup directory (e.g. `data.db` and `data-old.db` both defaulting to `<dir>/backup`) never delete each other's archives.

The timestamp is zero-padded to milliseconds, so names sort lexicographically in chronological order; two backups inside the same millisecond append a counter. After each run, archives beyond `retention` are deleted, oldest first.

## Restoring a Backup

Backups are plain SQLite database files wrapped in gzip. Decompress to stdout and redirect to the destination (`gunzip` has no `-o` flag on GNU systems, and this form preserves the archive):

```bash
gunzip -c data-20261016-150405-000.db.gz > restored.db
```

Point `orm.db_path` at the restored file (or replace the live database file while the process is stopped), then start the application.

## High-Load Skipping and Forced Runs

Before every scheduled run, the backup samples total CPU usage (~200ms). If usage is at or above `high_load_threshold`, the run is skipped: a warning is logged, the skip is recorded in the statistics (`last_skip_time`, `last_skip_reason`, `skip_count`), and the next-run projection advances. Failing to read CPU usage never blocks a backup.

To back up a busy node anyway, force a run — the forced path bypasses the load gate:

```bash
curl -XPOST .../sqlite/backup/_run
```

```json
{
  "forced": true,
  "last_backup_success": true,
  "last_backup_time": "2026-10-20T16:00:00+08:00",
  "last_backup_duration_ms": 1200,
  "last_backup_size": 20480,
  "last_backup_file": "data-20261020-160000-123.db.gz",
  "success_count": 3,
  "failure_count": 0
}
```

## Backup Statistics

`GET /sqlite/backup/stats` reports the backup configuration, the outcome of the last attempt, the next scheduled run, and cumulative counts:

```json
{
  "enabled": true,
  "db_path": "data/sqlite/data.db",
  "interval": "12h0m0s",
  "dir": "data/sqlite/backup",
  "retention": 14,
  "last_backup_time": "2026-10-16T15:04:05+08:00",
  "last_backup_duration_ms": 1200,
  "last_backup_size": 20480,
  "last_backup_file": "data-20261016-150405-000.db.gz",
  "last_backup_success": true,
  "last_backup_error": "",
  "last_success_time": "2026-10-16T15:04:05+08:00",
  "next_backup_time": "2026-10-17T03:04:06+08:00",
  "success_count": 10,
  "failure_count": 0,
  "last_skip_time": null,
  "last_skip_reason": "",
  "skip_count": 0
}
```

| Field                     | Description                                                        |
|---------------------------|--------------------------------------------------------------------|
| `last_backup_time`        | When the last attempt started (`null` before the first attempt).   |
| `last_backup_duration_ms` | Wall-clock duration of the last attempt.                           |
| `last_backup_size`        | Compressed size in bytes of the last successful archive.           |
| `last_backup_file`        | Path of the last successful archive.                               |
| `last_backup_success`     | Outcome of the last attempt.                                       |
| `last_backup_error`       | Error message when the last attempt failed, empty otherwise.       |
| `next_backup_time`        | Approximate time of the next scheduled run.                        |
| `success_count` / `failure_count` | Cumulative attempt counters since process start.          |

## Health Reporting

Every backup attempt reports the `sqlite_backup` service to the framework health registry (see `GET /health`):

- **green** — the last backup succeeded
- **red** — the last backup failed (disk full, permission errors, corruption…)

A failed backup turns the overall health status red, since a broken backup usually means the node cannot recover its metadata after a failure. The service appears in `/health` only after the first attempt.

## Permissions

On the web port, `GET /sqlite/backup/stats` requires login plus the dedicated permission key:

```
generic#system:sqlite-backup/read    # GET  /sqlite/backup/stats
generic#system:sqlite-backup/update  # POST /sqlite/backup/_run (forced backup)
```

Grant it to roles that need to inspect backup status:

```yaml
web:
  security:
    authorization:
      static:
        roles:
          - name: operator
            permissions:
              - "generic#system:sqlite-backup/read"
```

Managed instances include this key in their self API token automatically, so manager-originated calls (reverse channel, direct access) pass the permission filter without extra configuration.
