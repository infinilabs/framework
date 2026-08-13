/* Copyright © INFINI LTD. All rights reserved. */

package sqlite

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"

	log "github.com/cihub/seelog"

	"infini.sh/framework/core/global"
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

// createExpressionIndexes walks the model struct — recursing into nested
// object structs — and creates a SQLite expression index for every
// elastic_mapping field whose mapping type is B-tree-friendly.
//
// The index expression mirrors what the ORM's query builder emits
// (json_extract(raw,'$.field') for top-level fields and
// json_extract(raw,'$.parent.child') for nested ones), so existing
// TermQuery / Range / Sort clauses transparently use the index — including
// those on nested object fields — with no query changes required. SQLite
// supports indexes on expressions since 3.9 (2015); the modernc driver
// (pure-Go upstream SQLite) satisfies this.
//
// This is the key optimization that turns the JSON-blob storage model from
// full-table scans into index lookups: without it every WHERE/ORDER BY
// compiles to json_extract applied per-row.
func createExpressionIndexes(db *sql.DB, tableName string, model interface{}) {
	t := reflect.TypeOf(model)
	for t != nil && t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		return
	}
	tableSafe := sanitizeForIndexName(tableName)
	walkMappingIndexes(db, tableName, tableSafe, t, "")
}

// walkMappingIndexes recurses through the struct's fields. jsonPathPrefix is
// the dotted JSON path of the enclosing object ("" at the top level); each
// indexed field's path is prefix+"."+jsonField, matching what the query
// builder emits and what json_extract expects.
func walkMappingIndexes(db *sql.DB, tableName, tableSafe string, t reflect.Type, jsonPathPrefix string) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := strings.TrimSpace(field.Tag.Get("elastic_mapping"))
		if tag == "-" {
			continue
		}

		// An anonymous embedded struct with no mapping tag of its own has its
		// fields JSON-promoted to this level (e.g. orm.ORMObjectBase), so
		// recurse at the same path rather than introducing a path segment.
		if field.Anonymous && tag == "" {
			if ft := objectStructType(field.Type); ft != nil {
				walkMappingIndexes(db, tableName, tableSafe, ft, jsonPathPrefix)
			}
			continue
		}
		if tag == "" {
			continue
		}
		if mappingDisabled(tag) {
			continue
		}

		jsonField, esType, ok := parseMappingTag(tag)
		if jsonField == "" {
			continue
		}
		path := jsonField
		if jsonPathPrefix != "" {
			path = jsonPathPrefix + "." + jsonField
		}

		if ok && indexableElasticTypes[esType] {
			createOneExpressionIndex(db, tableName, tableSafe, path)
		}

		// Recurse into nested object structs so their scalar leaves get dotted
		// indexes ($.parent.child). Slices/arrays/maps are intentionally NOT
		// recursed: json_extract cannot address fields inside a JSON array via
		// a dotted path, and map values have no declared struct tags to index.
		if ft := objectStructType(field.Type); ft != nil {
			walkMappingIndexes(db, tableName, tableSafe, ft, path)
		}
	}
}

// createOneExpressionIndex issues CREATE INDEX for a single dotted JSON path.
func createOneExpressionIndex(db *sql.DB, tableName, tableSafe, jsonPath string) {
	idxName := indexNameFor(tableSafe, jsonPath)
	ddl := fmt.Sprintf(`CREATE INDEX IF NOT EXISTS [%s] ON [%s](json_extract(raw, '$.%s'))`,
		idxName, tableName, jsonPath)
	if _, err := db.Exec(ddl); err != nil {
		// Non-fatal: a bad index shouldn't block schema registration.
		log.Warnf("sqlite expression index %s on %s: %v", idxName, tableName, err)
	} else if global.Env().IsDebug {
		log.Debug("sqlite expression index: ", ddl)
	}
}

// indexNameFor builds a SQLite-safe index name from the table and the dotted
// JSON path, e.g. "ix_host_cpu_info_model". Dots and dashes become underscores.
func indexNameFor(tableSafe, jsonPath string) string {
	pathSafe := strings.NewReplacer("-", "_", ".", "_").Replace(jsonPath)
	return fmt.Sprintf("ix_%s_%s", tableSafe, pathSafe)
}

// parseMappingTag extracts the JSON field name and the ES type from an
// elastic_mapping tag fragment. Accepts both compact ("stream_id:{type:keyword}")
// and spaced ("created: { type: date }") forms.
//
// Only the field's own "type:" — the one sitting directly inside its mapping
// object (brace depth 1) — is honored. A "type:" nested inside a `properties:`
// sub-block describes a child field, not this one, so it is ignored; otherwise
// an object declared via `properties:` would be mistaken for a scalar.
//
// Returns ok=false when the tag has no top-level "type:" segment (e.g.
// nested-only mappings), so the caller can skip it as a scalar index (it may
// still be recursed into as an object).
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

	depth := 0
	for i := 0; i < len(rest); i++ {
		switch rest[i] {
		case '{':
			depth++
		case '}':
			if depth > 0 {
				depth--
			}
		}
		if depth != 1 || !strings.HasPrefix(rest[i:], "type:") {
			continue
		}
		// Word boundary: "type:" must be a key, not a suffix like "subtype:".
		if i > 0 && !isMappingSep(rest[i-1]) {
			continue
		}
		after := strings.TrimSpace(rest[i+len("type:"):])
		// The type token runs until a delimiter (',', '}', or whitespace).
		for j, c := range after {
			if c == ',' || c == '}' || c == ' ' || c == '\t' {
				esType = after[:j]
				break
			}
		}
		if esType == "" {
			esType = after
		}
		return jsonField, esType, true
	}
	return jsonField, "", false
}

// isMappingSep reports whether b can precede a mapping key like "type:".
func isMappingSep(b byte) bool {
	return b == ' ' || b == '\t' || b == '{' || b == ','
}

// mappingDisabled reports whether an elastic_mapping tag fragment disables
// indexing for the subtree, e.g. "payload:{type:object,enabled:false}". In ES
// such fields are stored but never parsed into individually addressable
// sub-fields, so neither they nor their children are indexable.
func mappingDisabled(tag string) bool {
	return strings.Contains(tag, "enabled:false") || strings.Contains(tag, "enabled: false")
}

// objectStructType returns the underlying struct type for a field that is a
// struct or a pointer to a struct; returns nil for slices, maps, arrays,
// interfaces and scalars.
func objectStructType(t reflect.Type) reflect.Type {
	for t != nil && t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t != nil && t.Kind() == reflect.Struct {
		return t
	}
	return nil
}

// sanitizeForIndexName replaces characters that are awkward in SQLite
// identifier names (table names like "logpilot-patterns" contain dashes).
func sanitizeForIndexName(s string) string {
	return strings.ReplaceAll(s, "-", "_")
}
