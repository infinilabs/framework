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
// it under the terms of the License as published by the Free Software
// Foundation, either version 3 of the License, or (at your option) any later
// version. See the GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program. If not, see <http://www.gnu.org/licenses/>.

package orm

import (
	"fmt"
	"regexp"
	"strings"
	"sync"

	log "github.com/cihub/seelog"
)

// Field names and sort directions end up inside SQL text (json_extract
// paths, ORDER BY fragments): they cannot be bound as query parameters.
// Every such identifier therefore passes through the strict validators in
// this file. Anything that fails validation degrades to a neutral
// expression (NULL / ASC) so a hostile name can only ever produce an
// empty/neutral query — never a syntax breakout.

// jsonPathRe matches the identifiers the query layer legitimately produces:
// dotted JSON paths of registered model fields, optionally with numeric
// array indices (foo.bar[0]). Model fields never need quotes, spaces,
// operators, or comment syntax — anything outside this set is hostile or
// garbage.
var jsonPathRe = regexp.MustCompile(`^[A-Za-z0-9_\-.\[\]]+$`)

// invalidPathOnce deduplicates warnings: a poller or attacker replaying the
// same bad identifier must not flood the log.
var invalidPathOnce sync.Map

// SafeJSONPathExpr renders path as a json_extract expression against the
// document column. Invalid or empty paths return the literal NULL, which
// makes predicates on them match nothing (identical to filtering on a
// nonexistent field) and ORDER BY / GROUP BY degrade to a constant — the
// same observable behavior as before, minus the injection.
func SafeJSONPathExpr(path string) string {
	if path == "" || !jsonPathRe.MatchString(path) {
		if _, loaded := invalidPathOnce.LoadOrStore(path, true); !loaded {
			log.Warnf("sqlite orm: rejected invalid JSON path identifier %q (possible injection attempt); "+
				"the clause degrades to NULL and matches nothing", path)
		}
		return "NULL"
	}
	return fmt.Sprintf("json_extract(raw, '$.%s')", path)
}

// SafeSortDirection whitelists the ORDER BY direction token. Empty input
// defaults to ASC; anything that is not exactly ASC/DESC (case-insensitive)
// also degrades to ASC instead of being spliced into the SQL text.
func SafeSortDirection(dir string) string {
	switch strings.ToUpper(strings.TrimSpace(dir)) {
	case "DESC":
		return "DESC"
	default:
		return "ASC"
	}
}
