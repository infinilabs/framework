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
	"net/http"

	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/security"
	"infini.sh/framework/core/util"
)

// activeORM holds the running module's handler so the stats endpoint can
// reach it; nil when the module (or backup) never started.
var activeORM *SQLiteORM

type API struct {
	api.Handler
}

func init() {
	module := API{}
	api.HandleAPIMethod(api.GET, "/sqlite/backup/stats", module.BackupStatsAction)
	api.HandleUIMethod(api.GET, "/sqlite/backup/stats", module.BackupStatsAction,
		api.RequireLogin(), api.RequirePermission(security.PermissionSystemSqliteBackupRead))
}

// BackupStatsAction reports backup configuration plus the last attempt
// (time, duration, size, outcome), the next scheduled run, and cumulative
// success/failure counts. The same outcome drives the sqlite_backup entry
// in /health.
func (module *API) BackupStatsAction(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
	if activeORM == nil {
		module.WriteJSON(w, util.MapStr{"enabled": false}, http.StatusOK)
		return
	}

	handler := activeORM
	stats := handler.GetBackupStats()
	data := util.MapStr{
		"enabled":                 true,
		"db_path":                 handler.Config.DBPath,
		"interval":                handler.Config.Backup.Interval.String(),
		"dir":                     handler.backupDir(),
		"retention":               handler.Config.Backup.Retention,
		"last_backup_time":        stats.LastAttemptTime,
		"last_backup_duration_ms": stats.LastDurationMs,
		"last_backup_size":        stats.LastSizeBytes,
		"last_backup_file":        stats.LastBackupFile,
		"last_backup_success":     stats.LastSuccess,
		"last_backup_error":       stats.LastError,
		"last_success_time":       stats.LastSuccessTime,
		"next_backup_time":        stats.NextBackupTime,
		"success_count":           stats.SuccessCount,
		"failure_count":           stats.FailureCount,
	}
	module.WriteJSON(w, data, http.StatusOK)
}
