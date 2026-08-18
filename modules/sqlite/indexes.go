/* Copyright © INFINI LTD. All rights reserved. */

package sqlite

import (
	"fmt"
	"reflect"
	"strings"
)

// indexableElasticTypes lists the elastic_mapping types promoted to SQLite
// generated columns (equality / range / sort). text is handled separately
// via FTS5; object/binary are excluded (no scalar column expression).
// Superseded as an index strategy by flattened.go, kept as the shared
// mapping-type vocabulary.
var indexableElasticTypes = map[string]bool{
	"keyword": true,
	"date":    true,
	"long":    true,
	"integer": true,
	"boolean": true,
	"double":  true,
	"float":   true,
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
// identifier names: dashes in table names ("logpilot-patterns") and dots in
// dotted JSON paths ("basic_auth.username") used for index/FTS column names.
func sanitizeForIndexName(s string) string {
	return strings.NewReplacer("-", "_", ".", "_").Replace(s)
}
