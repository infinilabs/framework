/* Copyright © INFINI LTD. All rights reserved. */

package sqlite

import (
	"database/sql"
	"fmt"
	"reflect"
	"strings"
	"sync"

	log "github.com/cihub/seelog"

	"infini.sh/framework/core/global"
	sqliteOrm "infini.sh/framework/modules/sqlite/orm"
)

// ──────────────────────────────────────────────────────────────────────────
// Flattened storage model.
//
// Tables keep the document shape — (id, raw JSON) — as the source of truth,
// while mapped scalar leaves are promoted to VIRTUAL generated columns:
//
//	"status" TEXT GENERATED ALWAYS AS (json_extract(raw,'$.status')) VIRTUAL
//
// Queries then hit real columns (plain B-tree indexes, composites possible,
// planner-friendly) instead of json_extract expression indexes. Generated
// columns are computed by SQLite itself, so the write path stays id-only +
// raw and can never drift. Text-mapped fields additionally get an FTS5
// external-content table kept in sync by triggers (match queries use MATCH).
//
// Benchmark context (200k rows, see SEARCH_ORM_REFACTOR_PLAN appendix D):
// filter+sort+LIMIT via composite index ~59ms → ~12µs; two-field AND+range
// ~68ms → ~5.9ms; single-field lookups and GROUP BY are on par.
// ──────────────────────────────────────────────────────────────────────────

// flattenedTypeAffinity maps elastic_mapping types to SQLite column
// affinities. json_extract returns TEXT for JSON strings, INTEGER for
// numbers/booleans, REAL for floats — matching these affinities keeps
// index comparisons type-correct.
var flattenedTypeAffinity = map[string]string{
	"keyword": "TEXT",
	"date":    "TEXT",
	"long":    "INTEGER",
	"integer": "INTEGER",
	"boolean": "INTEGER",
	"double":  "REAL",
	"float":   "REAL",
}

// fieldInfo is one mapped leaf found on the model struct.
type fieldInfo struct {
	Path   string // dotted JSON path, e.g. "basic_auth.username"
	ESType string // keyword/date/long/integer/boolean/double/float/text
}

// columnInfo describes one promoted generated column.
type columnInfo struct {
	Path     string // dotted JSON path (also the quoted column name)
	Affinity string // SQLite type affinity
	Expr     string // SQL expression referencing the column, e.g. ["status"]
}

// ftsInfo describes one FTS5-synced text field.
type ftsInfo struct {
	Path   string // dotted JSON path
	Column string // sanitized FTS column name (dots → underscores)
	Expr   string // SQL expression of the backing generated column
}

// tableSchema is the flattened layout derived from a registered model.
type tableSchema struct {
	Name      string
	Columns   []columnInfo // scalar leaves promoted to generated columns
	FTSFields []ftsInfo    // text leaves synced into the FTS table
	Composite [][]string   // composite index column lists (sqlite_composite tag)

	// lookup maps JSON path → column expression / FTS info for the resolver.
	columnByPath map[string]string
	ftsByPath    map[string]ftsInfo
	// dateEpochByPath maps date-mapped paths to their integer-epoch shadow
	// column expression — histogram bucketing does integer arithmetic on it
	// instead of parsing the RFC3339 text per row.
	dateEpochByPath map[string]string
}

// resolver returns the query-side plan for a JSON path: the comparison
// expression (generated column when promoted, json_extract fallback
// otherwise) and the FTS target when the path is a text field.
func (s *tableSchema) resolver() sqliteOrm.FieldResolver {
	return func(path string) (string, string, *sqliteOrm.FTSPlan) {
		if s == nil {
			return jsonExtractExpr(path), "", nil
		}
		if expr, ok := s.columnByPath[path]; ok {
			var fts *sqliteOrm.FTSPlan
			if f, ok := s.ftsByPath[path]; ok {
				fts = &sqliteOrm.FTSPlan{Table: ftsTableName(s.Name), Column: f.Column}
			}
			return expr, s.dateEpochByPath[path], fts
		}
		if f, ok := s.ftsByPath[path]; ok {
			return f.Expr, "", &sqliteOrm.FTSPlan{Table: ftsTableName(s.Name), Column: f.Column}
		}
		return jsonExtractExpr(path), "", nil
	}
}

func jsonExtractExpr(path string) string {
	return fmt.Sprintf("json_extract(raw, '$.%s')", path)
}

// registry of flattened schemas by table name.
var (
	schemaMu  sync.RWMutex
	schemaReg = map[string]*tableSchema{}
)

func registerTableSchema(s *tableSchema) {
	schemaMu.Lock()
	defer schemaMu.Unlock()
	schemaReg[s.Name] = s
}

func lookupTableSchema(table string) *tableSchema {
	schemaMu.RLock()
	defer schemaMu.RUnlock()
	return schemaReg[table]
}

// collectFields walks the model struct — recursing into nested object
// structs and anonymous embeds — collecting every elastic_mapping leaf.
// Mirrors the promotion rules: scalar leaves become generated columns,
// text leaves also join the FTS table. Slices/maps are not recursed
// (json_extract cannot address array elements via dotted paths).
func collectFields(model interface{}) (scalars, texts []fieldInfo, composites [][]string) {
	t := reflect.TypeOf(model)
	for t != nil && t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	if t == nil || t.Kind() != reflect.Struct {
		return nil, nil, nil
	}
	walkFields(t, "", &scalars, &texts, &composites)
	return scalars, texts, composites
}

func walkFields(t reflect.Type, prefix string, scalars, texts *[]fieldInfo, composites *[][]string) {
	for i := 0; i < t.NumField(); i++ {
		field := t.Field(i)
		tag := strings.TrimSpace(field.Tag.Get("elastic_mapping"))
		if tag == "-" {
			continue
		}

		// Composite index declarations: any field may carry a
		// sqlite_composite:"a,b" tag listing column paths for one
		// composite B-tree index (explicit opt-in, never inferred).
		if comp := strings.TrimSpace(field.Tag.Get("sqlite_composite")); comp != "" {
			var cols []string
			for _, p := range strings.Split(comp, ",") {
				if p = strings.TrimSpace(p); p != "" {
					cols = append(cols, p)
				}
			}
			if len(cols) > 1 {
				*composites = append(*composites, cols)
			}
		}

		// Anonymous embedded struct without its own mapping tag: JSON-promoted
		// fields (e.g. orm.ORMObjectBase) — recurse at the same path.
		if field.Anonymous && tag == "" {
			if ft := objectStructType(field.Type); ft != nil {
				walkFields(ft, prefix, scalars, texts, composites)
			}
			continue
		}
		if tag == "" || mappingDisabled(tag) {
			continue
		}

		jsonField, esType, ok := parseMappingTag(tag)
		if jsonField == "" {
			continue
		}
		path := jsonField
		if prefix != "" {
			path = prefix + "." + jsonField
		}

		if ok {
			if flattenedTypeAffinity[esType] != "" {
				*scalars = append(*scalars, fieldInfo{Path: path, ESType: esType})
			} else if esType == "text" {
				*texts = append(*texts, fieldInfo{Path: path, ESType: esType})
			}
		}

		if ft := objectStructType(field.Type); ft != nil {
			walkFields(ft, path, scalars, texts, composites)
		}
	}
}

// buildTableSchema derives the flattened layout for a model.
func buildTableSchema(tableName string, model interface{}) *tableSchema {
	scalars, texts, composites := collectFields(model)
	s := &tableSchema{
		Name:            tableName,
		Columns:         make([]columnInfo, 0, len(scalars)),
		FTSFields:       make([]ftsInfo, 0, len(texts)),
		Composite:       composites,
		columnByPath:    map[string]string{},
		ftsByPath:       map[string]ftsInfo{},
		dateEpochByPath: map[string]string{},
	}
	for _, f := range scalars {
		// "id" is the table's primary key and "raw" the document column —
		// promoting either would collide with the real column.
		if f.Path == "id" || f.Path == "raw" {
			continue
		}
		col := columnInfo{
			Path:     f.Path,
			Affinity: flattenedTypeAffinity[f.ESType],
			Expr:     quoteIdent(f.Path),
		}
		s.Columns = append(s.Columns, col)
		s.columnByPath[f.Path] = col.Expr
		// Date fields get an integer-epoch shadow for bucketing math.
		if f.ESType == "date" {
			epochPath := dateEpochColumn(f.Path)
			s.Columns = append(s.Columns, columnInfo{
				Path:     epochPath,
				Affinity: "INTEGER",
				Expr:     quoteIdent(epochPath),
			})
			s.dateEpochByPath[f.Path] = quoteIdent(epochPath)
		}
	}
	// The PK column serves id-path queries directly.
	s.columnByPath["id"] = quoteIdent("id")
	// Text fields are materialized as generated columns too — the FTS
	// triggers read them (verified: triggers may reference VIRTUAL columns).
	for _, f := range texts {
		fts := ftsInfo{
			Path:   f.Path,
			Column: sanitizeForIndexName(f.Path),
			Expr:   quoteIdent(f.Path),
		}
		s.FTSFields = append(s.FTSFields, fts)
		s.ftsByPath[f.Path] = fts
	}
	return s
}

// ensureFlattenedTable creates or migrates the table to the flattened
// layout: generated columns inline, plain column indexes, FTS sync.
// Migration is a transactional rebuild when an existing table predates
// generated columns (they cannot be added via ALTER TABLE).
func ensureFlattenedTable(db *sql.DB, s *tableSchema) error {
	rebuilt, err := createOrMigrateTable(db, s)
	if err != nil {
		return err
	}
	dropLegacyExpressionIndexes(db, s)
	createColumnIndexes(db, s)
	if rebuilt && len(s.FTSFields) > 0 {
		// The rebuild reassigned rowids and dropped the old table's sync
		// triggers — drop the stale FTS index so ensureFTS recreates and
		// backfills it against the new rowids.
		if _, err := db.Exec(fmt.Sprintf("DROP TABLE IF EXISTS [%s]", ftsTableName(s.Name))); err != nil {
			log.Warnf("sqlite: drop stale FTS table for %s: %v", s.Name, err)
		}
	}
	ensureFTS(db, s)
	return nil
}

// createOrMigrateTable creates the table with generated columns, or
// rebuilds an older layout in place (same table name, data preserved).
// Returns rebuilt=true when a migration replaced the table (rowids change
// and the old table's triggers die, so the FTS index must be rebuilt).
func createOrMigrateTable(db *sql.DB, s *tableSchema) (bool, error) {
	// Build the column DDL fragment once: reused by create and rebuild.
	var colDefs []string
	// STORED (not VIRTUAL): table scans and aggregate reads must not
	// re-evaluate json_extract — a full-document JSON parse — per row per
	// column. VIRTUAL is only free for index-only lookups (the index
	// materializes the value); scans pay O(rows × columns) parses, which
	// measured ~10s for a 3-level aggregation over 500k rows. The one-time
	// write-time materialization is negligible for metadata-scale stores.
	for _, c := range s.Columns {
		if expr, ok := epochSourcePath(c.Path); ok {
			// Integer-epoch shadow of a date field: histogram bucketing does
			// arithmetic on it instead of parsing the RFC3339 text per row.
			colDefs = append(colDefs, fmt.Sprintf("%s INTEGER GENERATED ALWAYS AS (CAST(strftime('%%s', json_extract(raw, '$.%s')) AS INTEGER)) STORED",
				quoteIdent(c.Path), expr))
			continue
		}
		colDefs = append(colDefs, fmt.Sprintf("%s %s GENERATED ALWAYS AS (json_extract(raw, '$.%s')) STORED",
			quoteIdent(c.Path), c.Affinity, c.Path))
	}
	for _, f := range s.FTSFields {
		colDefs = append(colDefs, fmt.Sprintf("%s TEXT GENERATED ALWAYS AS (json_extract(raw, '$.%s')) STORED",
			quoteIdent(f.Path), f.Path))
	}
	colsDDL := ""
	if len(colDefs) > 0 {
		colsDDL = ", " + strings.Join(colDefs, ", ")
	}

	exists, err := tableExists(db, s.Name)
	if err != nil {
		return false, err
	}
	if !exists {
		ddl := fmt.Sprintf("CREATE TABLE [%s] (id TEXT PRIMARY KEY, raw JSON NOT NULL%s)", s.Name, colsDDL)
		if _, err := db.Exec(ddl); err != nil {
			return false, fmt.Errorf("failed to create table %s: %w", s.Name, err)
		}
		if global.Env().IsDebug {
			log.Debug("sqlite DDL: ", ddl)
		}
		return false, nil
	}

	// Existing table: check whether every expected column is present.
	have, err := tableColumns(db, s.Name)
	if err != nil {
		return false, err
	}
	missing := false
	for _, c := range s.Columns {
		if !have[c.Path] {
			missing = true
			break
		}
	}
	if !missing {
		for _, f := range s.FTSFields {
			if !have[f.Path] {
				missing = true
				break
			}
		}
	}
	if !missing {
		return false, nil
	}

	// Generated columns cannot be ALTERed in — rebuild the table inside a
	// transaction: create shadow with the new layout, copy id+raw, swap.
	log.Infof("sqlite: migrating table %s to flattened layout (%d generated columns)", s.Name, len(s.Columns)+len(s.FTSFields))
	tx, err := db.Begin()
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	shadow := s.Name + "__migrate"
	if _, err := tx.Exec(fmt.Sprintf("DROP TABLE IF EXISTS [%s]", shadow)); err != nil {
		return false, err
	}
	ddl := fmt.Sprintf("CREATE TABLE [%s] (id TEXT PRIMARY KEY, raw JSON NOT NULL%s)", shadow, colsDDL)
	if _, err := tx.Exec(ddl); err != nil {
		return false, fmt.Errorf("failed to create migration table for %s: %w", s.Name, err)
	}
	if _, err := tx.Exec(fmt.Sprintf("INSERT INTO [%s] (id, raw) SELECT id, raw FROM [%s]", shadow, s.Name)); err != nil {
		return false, err
	}
	if _, err := tx.Exec(fmt.Sprintf("DROP TABLE [%s]", s.Name)); err != nil {
		return false, err
	}
	if _, err := tx.Exec(fmt.Sprintf("ALTER TABLE [%s] RENAME TO [%s]", shadow, s.Name)); err != nil {
		return false, err
	}
	if err := tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}

// dropLegacyExpressionIndexes removes the pre-flattening json_extract
// expression indexes (name scheme ix_<table>_<path>) so planner choices
// move to the new column indexes.
func dropLegacyExpressionIndexes(db *sql.DB, s *tableSchema) {
	tableSafe := sanitizeForIndexName(s.Name)
	for _, c := range s.Columns {
		dropIndexIfKnown(db, indexNameFor(tableSafe, c.Path))
	}
	for _, f := range s.FTSFields {
		dropIndexIfKnown(db, indexNameFor(tableSafe, f.Path))
	}
}

func dropIndexIfKnown(db *sql.DB, name string) {
	if _, err := db.Exec(fmt.Sprintf("DROP INDEX IF EXISTS [%s]", name)); err != nil {
		log.Warnf("sqlite: drop legacy index %s: %v", name, err)
	}
}

// createColumnIndexes builds plain B-tree indexes on promoted columns and
// the declared composites. All promoted scalars get a single-column index
// (parity with the old expression-index coverage).
func createColumnIndexes(db *sql.DB, s *tableSchema) {
	tableSafe := sanitizeForIndexName(s.Name)
	for _, c := range s.Columns {
		if _, isEpoch := epochSourcePath(c.Path); isEpoch {
			// Epoch shadows back the aggregation math only; the TEXT date
			// index already covers equality/range queries.
			continue
		}
		idxName := "ixc_" + tableSafe + "_" + sanitizeForIndexName(c.Path)
		ddl := fmt.Sprintf("CREATE INDEX IF NOT EXISTS [%s] ON [%s](%s)", idxName, s.Name, c.Expr)
		if _, err := db.Exec(ddl); err != nil {
			log.Warnf("sqlite column index %s: %v", idxName, err)
		} else if global.Env().IsDebug {
			log.Debug("sqlite column index: ", ddl)
		}
	}
	for i, cols := range s.Composite {
		var exprs []string
		var nameParts []string
		for _, p := range cols {
			// Column names are used verbatim: TEXT date columns keep their
			// lexicographic ordering (walk/sort plans depend on it); covering
			// aggregation composites may reference the integer epoch shadow
			// explicitly, e.g. "bucket_start__epoch".
			exprs = append(exprs, quoteIdent(p))
			nameParts = append(nameParts, sanitizeForIndexName(p))
		}
		idxName := fmt.Sprintf("ixcc_%s_%d_%s", tableSafe, i, strings.Join(nameParts, "_"))
		ddl := fmt.Sprintf("CREATE INDEX IF NOT EXISTS [%s] ON [%s](%s)", idxName, s.Name, strings.Join(exprs, ", "))
		if _, err := db.Exec(ddl); err != nil {
			log.Warnf("sqlite composite index %s: %v", idxName, err)
		} else if global.Env().IsDebug {
			log.Debug("sqlite composite index: ", ddl)
		}
	}
}

// ftsTableName is the FTS5 external-content table name for a table.
func ftsTableName(table string) string {
	return "fts_" + sanitizeForIndexName(table)
}

// ensureFTS creates the FTS5 external-content table plus AI/AU/AD triggers
// for text-mapped fields, and backfills existing rows. Column names are
// sanitized (dots → underscores); triggers reference the generated columns
// explicitly, so no name correspondence with the content table is needed.
// Failure degrades to LIKE-based search (resolver skips the FTS plan).
func ensureFTS(db *sql.DB, s *tableSchema) {
	if len(s.FTSFields) == 0 {
		return
	}
	fts := ftsTableName(s.Name)

	// Only backfill rows that predate the FTS table; probing for existence
	// first avoids the full anti-join scan on every boot (the backfill
	// SELECT runs a content-table scan otherwise).
	existed := ftsAvailable(db, s.Name)

	var cols []string
	for _, f := range s.FTSFields {
		cols = append(cols, quoteIdent(f.Column))
	}
	ddl := fmt.Sprintf("CREATE VIRTUAL TABLE IF NOT EXISTS [%s] USING fts5(%s, content='%s', content_rowid='rowid')",
		fts, strings.Join(cols, ", "), s.Name)
	if _, err := db.Exec(ddl); err != nil {
		log.Warnf("sqlite FTS5 table %s: %v (falling back to LIKE search)", fts, err)
		return
	}
	if global.Env().IsDebug {
		log.Debug("sqlite FTS5: ", ddl)
	}
	if existed {
		return // triggers are IF NOT EXISTS; no new rows to backfill
	}

	var colList, newCols, oldCols, plainCols string
	for i, f := range s.FTSFields {
		if i > 0 {
			colList += ", "
			newCols += ", "
			oldCols += ", "
			plainCols += ", "
		}
		colList += quoteIdent(f.Column)
		newCols += fmt.Sprintf("new.%s", quoteIdent(f.Path))
		oldCols += fmt.Sprintf("old.%s", quoteIdent(f.Path))
		plainCols += quoteIdent(f.Path) // the generated columns themselves
	}

	statements := []string{
		// AI: index the new row's text columns.
		fmt.Sprintf("CREATE TRIGGER IF NOT EXISTS [%[1]s_ai] AFTER INSERT ON [%[2]s] BEGIN INSERT INTO [%[1]s](rowid, %[3]s) VALUES (new.rowid, %[4]s); END",
			fts, s.Name, colList, newCols),
		// AU: FTS5 external-content delete of the old values, then insert
		// of the new ones ('delete' is a command row, not a real insert).
		fmt.Sprintf("CREATE TRIGGER IF NOT EXISTS [%[1]s_au] AFTER UPDATE ON [%[2]s] BEGIN INSERT INTO [%[1]s]([%[1]s], rowid, %[3]s) VALUES('delete', old.rowid, %[4]s); INSERT INTO [%[1]s](rowid, %[3]s) VALUES (new.rowid, %[5]s); END",
			fts, s.Name, colList, oldCols, newCols),
		// AD: remove the deleted row's entries.
		fmt.Sprintf("CREATE TRIGGER IF NOT EXISTS [%[1]s_ad] AFTER DELETE ON [%[2]s] BEGIN INSERT INTO [%[1]s]([%[1]s], rowid, %[3]s) VALUES('delete', old.rowid, %[4]s); END",
			fts, s.Name, colList, oldCols),
	}
	for _, stmt := range statements {
		if _, err := db.Exec(stmt); err != nil {
			log.Warnf("sqlite FTS5 trigger on %s: %v", s.Name, err)
			return
		}
	}

	// Backfill rows inserted before the FTS table existed. The automatic
	// 'rebuild' command relies on column-name correspondence with the
	// content table, which sanitized names break — populate explicitly
	// from the generated columns themselves.
	backfill := fmt.Sprintf("INSERT INTO [%s](rowid, %s) SELECT rowid, %s FROM [%s] WHERE rowid NOT IN (SELECT rowid FROM [%s])",
		fts, colList, plainCols, s.Name, fts)
	if _, err := db.Exec(backfill); err != nil {
		log.Warnf("sqlite FTS5 backfill for %s: %v", s.Name, err)
	} else if global.Env().IsDebug {
		log.Debug("sqlite FTS5 backfill: ", backfill)
	}
}

// ftsAvailable reports whether the FTS table exists (LIKE fallback decides
// on this at query time; a missing table means ensureFTS degraded).
func ftsAvailable(db *sql.DB, table string) bool {
	fts := ftsTableName(table)
	var name string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", fts).Scan(&name)
	return err == nil
}

func tableExists(db *sql.DB, table string) (bool, error) {
	var name string
	err := db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// tableColumns returns the set of column names of an existing table,
// including VIRTUAL generated columns (hidden from pragma_table_info but
// visible in pragma_table_xinfo).
func tableColumns(db *sql.DB, table string) (map[string]bool, error) {
	rows, err := db.Query("SELECT name FROM pragma_table_xinfo(?)", table)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	cols := map[string]bool{}
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		cols[name] = true
	}
	return cols, rows.Err()
}

// quoteIdent wraps an identifier in double quotes so dotted JSON paths
// work as column names ("basic_auth.username").
func quoteIdent(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}

// dateEpochColumn is the shadow-column name for a date path.
func dateEpochColumn(path string) string { return path + "__epoch" }

// epochSourcePath reverses dateEpochColumn: returns the source date path
// when the column is an epoch shadow.
func epochSourcePath(col string) (string, bool) {
	if strings.HasSuffix(col, "__epoch") {
		return strings.TrimSuffix(col, "__epoch"), true
	}
	return "", false
}
