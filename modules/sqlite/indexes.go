/* Copyright © INFINI LTD. All rights reserved. */

package sqlite

import (
	"database/sql"
	"fmt"
	"strings"

	log "github.com/cihub/seelog"

	"infini.sh/framework/core/global"
	"infini.sh/framework/core/util"
)

// indexableElasticTypes lists the elastic_mapping types that map well to a
// SQLite B-tree expression index (equality / range / sort). text is excluded
// (needs FTS5), object/binary are excluded (no scalar column expression).
var indexableElasticTypes = map[string]bool{
	"keyword": true,
	"date":    true,
	"long":    true,
	"integer": true,
	"boolean": true,
	"double":  true,
	"float":   true,
}

// createExpressionIndexes inspects the model's `elastic_mapping` struct tags
// and creates a SQLite expression index for each field whose mapping type is
// B-tree-friendly.
//
// The index expression mirrors what the ORM's query builder emits
// (json_extract(raw,'$.field')), so existing TermQuery / Range / Sort clauses
// transparently use the index — no query changes required. SQLite supports
// indexes on expressions since 3.9 (2015); the bundled ncruces driver
// satisfies this.
//
// This is the key optimization that turns the JSON-blob storage model from
// full-table scans into index lookups: without it every WHERE/ORDER BY
// compiles to json_extract applied per-row.
func createExpressionIndexes(db *sql.DB, tableName string, model interface{}) {
	annotations := util.GetTagsByTagName(model, "elastic_mapping")
	if len(annotations) == 0 {
		return
	}
	tableSafe := sanitizeForIndexName(tableName)
	for _, a := range annotations {
		jsonField, esType, ok := parseMappingTag(a.Tag)
		if !ok || jsonField == "" || !indexableElasticTypes[esType] {
			continue
		}
		idxName := fmt.Sprintf("ix_%s_%s", tableSafe, jsonField)
		ddl := fmt.Sprintf(`CREATE INDEX IF NOT EXISTS [%s] ON [%s](json_extract(raw, '$.%s'))`,
			idxName, tableName, jsonField)
		if _, err := db.Exec(ddl); err != nil {
			// Non-fatal: a bad index shouldn't block schema registration.
			log.Warnf("sqlite expression index %s on %s: %v", idxName, tableName, err)
		} else if global.Env().IsDebug {
			log.Debug("sqlite expression index: ", ddl)
		}
	}
}

// parseMappingTag extracts the JSON field name and the ES type from an
// elastic_mapping tag fragment. Accepts both compact ("stream_id:{type:keyword}")
// and spaced ("created: { type: date }") forms.
//
// Returns ok=false when the tag has no "type:" segment (e.g. nested-only
// mappings), so the caller can skip it.
func parseMappingTag(tag string) (jsonField, esType string, ok bool) {
	tag = strings.TrimSpace(tag)
	if tag == "" {
		return "", "", false
	}
	// The JSON field name is everything before the first ':'.
	colon := strings.Index(tag, ":")
	if colon < 0 {
		return "", "", false
	}
	jsonField = strings.TrimSpace(tag[:colon])
	rest := tag[colon+1:]
	ti := strings.Index(rest, "type:")
	if ti < 0 {
		return jsonField, "", false
	}
	after := strings.TrimSpace(rest[ti+len("type:"):])
	// The type token runs until a delimiter (',', '}', or whitespace).
	for i, c := range after {
		if c == ',' || c == '}' || c == ' ' || c == '\t' {
			esType = after[:i]
			break
		}
	}
	if esType == "" {
		esType = after
	}
	return jsonField, esType, true
}

// sanitizeForIndexName replaces characters that are awkward in SQLite
// identifier names (table names like "logpilot-patterns" contain dashes).
func sanitizeForIndexName(s string) string {
	return strings.ReplaceAll(s, "-", "_")
}
