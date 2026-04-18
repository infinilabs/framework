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
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	log "github.com/cihub/seelog"
	_ "github.com/ncruces/go-sqlite3/driver"
	_ "github.com/ncruces/go-sqlite3/embed"
	"infini.sh/framework/core/errors"
	"infini.sh/framework/core/global"
	api "infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	sqliteOrm "infini.sh/framework/modules/sqlite/orm"
)

var ErrNotFound = errors.New("record not found")

// SQLiteORM implements the orm.ORM interface using SQLite as the backend.
type SQLiteORM struct {
	Config SQLiteConfig
	DB     *sql.DB
}

// Open initializes the SQLite database connection.
func (handler *SQLiteORM) Open() error {
	// Ensure the parent directory exists
	dir := filepath.Dir(handler.Config.DBPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	db, err := sql.Open("sqlite3", handler.Config.DBPath)
	if err != nil {
		return fmt.Errorf("failed to open sqlite database at %s: %w", handler.Config.DBPath, err)
	}

	// Enable WAL mode for better concurrent read performance
	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		db.Close()
		return fmt.Errorf("failed to enable WAL mode: %w", err)
	}

	// Enable foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys=ON"); err != nil {
		db.Close()
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	handler.DB = db
	return nil
}

// Close closes the underlying database connection.
func (handler *SQLiteORM) Close() error {
	if handler.DB != nil {
		return handler.DB.Close()
	}
	return nil
}

func (handler *SQLiteORM) GetWildcardIndexName(o interface{}) string {
	return handler.GetIndexName(o)
}

func (handler *SQLiteORM) GetIndexName(o interface{}) string {
	return getTableName(o)
}

// RegisterSchemaWithName creates the table for the given struct type if it does not exist.
func (handler *SQLiteORM) RegisterSchemaWithName(t interface{}, indexName string) error {
	initTableName(t, indexName)
	tableName := handler.GetIndexName(t)

	ddl := fmt.Sprintf("CREATE TABLE IF NOT EXISTS [%s] (id TEXT PRIMARY KEY, raw JSON NOT NULL)", tableName)

	if global.Env().IsDebug {
		log.Debug("sqlite DDL: ", ddl)
	}

	_, err := handler.DB.Exec(ddl)
	if err != nil {
		return fmt.Errorf("failed to create table %s: %w", tableName, err)
	}
	log.Debugf("sqlite schema registered: %s", tableName)
	return nil
}

func (handler *SQLiteORM) Get(ctx *api.Context, o interface{}) (bool, error) {
	id := getObjectID(o)
	if id == "" {
		return false, errors.Errorf("id was not found in object: %v", o)
	}

	tableName := handler.GetIndexName(o)
	query := fmt.Sprintf("SELECT raw FROM [%s] WHERE id = ?", tableName)

	if global.Env().IsDebug {
		log.Debug("sqlite Get: ", query, " id=", id)
	}

	var rawJSON []byte
	err := handler.DB.QueryRow(query, id).Scan(&rawJSON)
	if err != nil {
		if err == sql.ErrNoRows {
			return false, ErrNotFound
		}
		return false, err
	}

	err = util.FromJSONBytes(rawJSON, o)
	return true, err
}

func (handler *SQLiteORM) Create(ctx *api.Context, o interface{}) error {
	id := getObjectID(o)
	if id == "" {
		return errors.New("id is required for create")
	}

	tableName := handler.GetIndexName(o)
	rawJSON := util.MustToJSONBytes(o)

	query := fmt.Sprintf("INSERT INTO [%s] (id, raw) VALUES (?, ?)", tableName)
	if global.Env().IsDebug {
		log.Debug("sqlite Create: ", query, " id=", id)
	}

	_, err := handler.DB.Exec(query, id, rawJSON)
	return err
}

func (handler *SQLiteORM) Save(ctx *api.Context, o interface{}) error {
	id := getObjectID(o)
	if id == "" {
		return errors.New("id is required for save")
	}

	tableName := handler.GetIndexName(o)
	rawJSON := util.MustToJSONBytes(o)

	query := fmt.Sprintf("INSERT OR REPLACE INTO [%s] (id, raw) VALUES (?, ?)", tableName)
	if global.Env().IsDebug {
		log.Debug("sqlite Save: ", query, " id=", id)
	}

	_, err := handler.DB.Exec(query, id, rawJSON)
	return err
}

func (handler *SQLiteORM) Update(ctx *api.Context, o interface{}) error {
	id := getObjectID(o)
	if id == "" {
		return errors.New("id is required for update")
	}

	tableName := handler.GetIndexName(o)
	rawJSON := util.MustToJSONBytes(o)

	query := fmt.Sprintf("UPDATE [%s] SET raw = ? WHERE id = ?", tableName)
	if global.Env().IsDebug {
		log.Debug("sqlite Update: ", query, " id=", id)
	}

	result, err := handler.DB.Exec(query, rawJSON, id)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (handler *SQLiteORM) Delete(ctx *api.Context, o interface{}) error {
	id := getObjectID(o)
	if id == "" {
		return errors.New("id is required for delete")
	}

	tableName := handler.GetIndexName(o)
	query := fmt.Sprintf("DELETE FROM [%s] WHERE id = ?", tableName)
	if global.Env().IsDebug {
		log.Debug("sqlite Delete: ", query, " id=", id)
	}

	_, err := handler.DB.Exec(query, id)
	return err
}

func (handler *SQLiteORM) DeleteBy(o interface{}, query interface{}) error {
	tableName := handler.GetIndexName(o)
	var queryBody []byte
	var ok bool
	if queryBody, ok = query.([]byte); !ok {
		return errors.New("type of param query should be byte array (raw SQL WHERE clause)")
	}

	sqlStr := fmt.Sprintf("DELETE FROM [%s] WHERE %s", tableName, string(queryBody))
	if global.Env().IsDebug {
		log.Debug("sqlite DeleteBy: ", sqlStr)
	}

	_, err := handler.DB.Exec(sqlStr)
	return err
}

func (handler *SQLiteORM) UpdateBy(o interface{}, query interface{}) error {
	tableName := handler.GetIndexName(o)
	var queryBody []byte
	var ok bool
	if queryBody, ok = query.([]byte); !ok {
		return errors.New("type of param query should be byte array (raw SQL statement)")
	}

	sqlStr := fmt.Sprintf("UPDATE [%s] SET %s", tableName, string(queryBody))
	if global.Env().IsDebug {
		log.Debug("sqlite UpdateBy: ", sqlStr)
	}

	_, err := handler.DB.Exec(sqlStr)
	return err
}

func (handler *SQLiteORM) Count(o interface{}, query interface{}) (int64, error) {
	tableName := handler.GetIndexName(o)

	sqlStr := fmt.Sprintf("SELECT COUNT(*) FROM [%s]", tableName)
	if query != nil {
		if queryBody, ok := query.([]byte); ok && len(queryBody) > 0 {
			sqlStr += " WHERE " + string(queryBody)
		}
	}

	if global.Env().IsDebug {
		log.Debug("sqlite Count: ", sqlStr)
	}

	var count int64
	err := handler.DB.QueryRow(sqlStr).Scan(&count)
	return count, err
}

func (handler *SQLiteORM) GetBy(field string, value interface{}, t interface{}) (error, api.Result) {
	query := api.Query{}
	query.Conds = api.And(api.Eq(field, value))
	return handler.Search(t, &query)
}

func (handler *SQLiteORM) Search(t interface{}, q *api.Query) (error, api.Result) {
	tableName := q.IndexName
	if tableName == "" {
		tableName = handler.GetIndexName(t)
	}

	result := api.Result{}

	whereClauses, whereArgs := buildLegacyWhere(q.Conds)
	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Get total count first
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM [%s]%s", tableName, whereSQL)
	var total int64
	if err := handler.DB.QueryRow(countSQL, whereArgs...).Scan(&total); err != nil {
		return err, result
	}

	sqlStr := fmt.Sprintf("SELECT raw FROM [%s]%s", tableName, whereSQL)

	if q.Sort != nil && len(*q.Sort) > 0 {
		var sorts []string
		for _, s := range *q.Sort {
			sorts = append(sorts, fmt.Sprintf("json_extract(raw, '$.%s') %s", s.Field, string(s.SortType)))
		}
		sqlStr += " ORDER BY " + strings.Join(sorts, ", ")
	}

	if q.Size > 0 {
		sqlStr += fmt.Sprintf(" LIMIT %d", q.Size)
	}
	if q.From > 0 {
		sqlStr += fmt.Sprintf(" OFFSET %d", q.From)
	}

	if global.Env().IsDebug {
		log.Debug("sqlite Search: ", sqlStr, " args=", whereArgs)
	}

	rows, err := handler.DB.Query(sqlStr, whereArgs...)
	if err != nil {
		return err, result
	}
	defer rows.Close()

	var array []interface{}
	for rows.Next() {
		var rawJSON []byte
		if err := rows.Scan(&rawJSON); err != nil {
			return err, result
		}
		var doc map[string]interface{}
		if err := util.FromJSONBytes(rawJSON, &doc); err != nil {
			return err, result
		}
		array = append(array, doc)
	}

	result.Result = array
	result.Total = total
	result.Raw = util.MustToJSONBytes(array)
	return nil, result
}

func (handler *SQLiteORM) SearchWithResultItemMapper(resultArray interface{}, itemMapFunc func(source map[string]interface{}, targetRef interface{}) error, q *api.Query) (error, *api.SimpleResult) {
	if q == nil {
		panic("invalid query")
	}

	arrayValue := reflect.ValueOf(resultArray)
	if arrayValue.Kind() != reflect.Ptr || arrayValue.Elem().Kind() != reflect.Slice {
		return fmt.Errorf("resultArray must be a pointer to a slice"), nil
	}

	sliceValue := arrayValue.Elem()
	elementType := sliceValue.Type().Elem()

	tableName := q.IndexName
	if tableName == "" {
		tempInstance := reflect.New(elementType).Interface()
		tableName = handler.GetIndexName(tempInstance)
	}

	whereClauses, whereArgs := buildLegacyWhere(q.Conds)
	whereSQL := ""
	if len(whereClauses) > 0 {
		whereSQL = " WHERE " + strings.Join(whereClauses, " AND ")
	}

	// Get total count first
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM [%s]%s", tableName, whereSQL)
	var total int64
	if err := handler.DB.QueryRow(countSQL, whereArgs...).Scan(&total); err != nil {
		return err, nil
	}

	sqlStr := fmt.Sprintf("SELECT raw FROM [%s]%s", tableName, whereSQL)

	if q.Sort != nil && len(*q.Sort) > 0 {
		var sorts []string
		for _, s := range *q.Sort {
			sorts = append(sorts, fmt.Sprintf("json_extract(raw, '$.%s') %s", s.Field, string(s.SortType)))
		}
		sqlStr += " ORDER BY " + strings.Join(sorts, ", ")
	}

	if q.Size > 0 {
		sqlStr += fmt.Sprintf(" LIMIT %d", q.Size)
	}
	if q.From > 0 {
		sqlStr += fmt.Sprintf(" OFFSET %d", q.From)
	}

	if global.Env().IsDebug {
		log.Debug("sqlite SearchWithResultItemMapper: ", sqlStr, " args=", whereArgs)
	}

	rows, err := handler.DB.Query(sqlStr, whereArgs...)
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	for rows.Next() {
		var rawJSON []byte
		if err := rows.Scan(&rawJSON); err != nil {
			return err, nil
		}

		elem := reflect.New(elementType).Elem()
		var source map[string]interface{}
		if err := util.FromJSONBytes(rawJSON, &source); err != nil {
			return err, nil
		}

		if itemMapFunc != nil {
			if err := itemMapFunc(source, elem.Addr().Interface()); err != nil {
				return fmt.Errorf("failed to map document to struct: %w", err), nil
			}
		}

		sliceValue.Set(reflect.Append(sliceValue, elem))
	}

	result := &api.SimpleResult{
		Total: total,
	}
	return nil, result
}

func (handler *SQLiteORM) GroupBy(t interface{}, selectField, groupField string, haveQuery string, haveValue interface{}) (error, map[string]interface{}) {
	tableName := handler.GetIndexName(t)

	sqlStr := fmt.Sprintf("SELECT json_extract(raw, '$.%s') as grp, COUNT(*) as cnt FROM [%s] GROUP BY grp",
		groupField, tableName)
	if haveQuery != "" {
		sqlStr += fmt.Sprintf(" HAVING %s", haveQuery)
	}

	if global.Env().IsDebug {
		log.Debug("sqlite GroupBy: ", sqlStr)
	}

	rows, err := handler.DB.Query(sqlStr)
	if err != nil {
		return err, nil
	}
	defer rows.Close()

	finalResult := map[string]interface{}{}
	for rows.Next() {
		var key string
		var count int64
		if err := rows.Scan(&key, &count); err != nil {
			return err, nil
		}
		finalResult[key] = count
	}
	return nil, finalResult
}

func (handler *SQLiteORM) SearchV2(ctx *api.Context, qb *api.QueryBuilder) (*api.SearchResult, error) {
	var indexName string
	if ctx != nil {
		if indices := api.GetIndices(ctx); len(indices) > 0 {
			indexName = indices[0]
		}
		if indexName == "" {
			if pattern := api.GetIndexPattern(ctx); pattern != "" {
				indexName = pattern
			}
		}
		if indexName == "" {
			model := api.GetModel(ctx)
			if model != nil {
				indexName = handler.GetIndexName(model)
			}
		}
	}
	if indexName == "" {
		return nil, errors.New("cannot resolve table name from context")
	}

	result := &api.SearchResult{}

	if qb != nil {
		qb.Build()
	}

	where, args := sqliteOrm.BuildWhereClause(qb)

	// Count total
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM [%s]", indexName)
	if where != "" {
		countSQL += " WHERE " + where
	}

	var total int64
	err := handler.DB.QueryRow(countSQL, args...).Scan(&total)
	if err != nil {
		return nil, err
	}

	// Build SELECT
	sqlStr := fmt.Sprintf("SELECT raw FROM [%s]", indexName)
	if where != "" {
		sqlStr += " WHERE " + where
	}

	if qb != nil {
		if sorts := qb.Sorts(); len(sorts) > 0 {
			var sortParts []string
			for _, s := range sorts {
				sortParts = append(sortParts, fmt.Sprintf("json_extract(raw, '$.%s') %s", s.Field, string(s.SortType)))
			}
			sqlStr += " ORDER BY " + strings.Join(sortParts, ", ")
		}

		if qb.SizeVal() > 0 {
			sqlStr += fmt.Sprintf(" LIMIT %d", qb.SizeVal())
		}
		if qb.FromVal() > 0 {
			sqlStr += fmt.Sprintf(" OFFSET %d", qb.FromVal())
		}
	}

	if global.Env().IsDebug {
		log.Debug("sqlite SearchV2: ", sqlStr, " args=", args)
	}

	rows, err := handler.DB.Query(sqlStr, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var docs []map[string]interface{}
	for rows.Next() {
		var rawJSON []byte
		if err := rows.Scan(&rawJSON); err != nil {
			return nil, err
		}
		var doc map[string]interface{}
		if err := util.FromJSONBytes(rawJSON, &doc); err != nil {
			return nil, err
		}
		docs = append(docs, doc)
	}

	// Build an Elasticsearch-compatible response structure
	hitsArray := make([]map[string]interface{}, 0, len(docs))
	for _, doc := range docs {
		id, _ := doc["id"].(string)
		hitsArray = append(hitsArray, map[string]interface{}{
			"_id":     id,
			"_source": doc,
		})
	}

	response := util.MapStr{
		"hits": util.MapStr{
			"total": util.MapStr{
				"value":    total,
				"relation": "eq",
			},
			"hits": hitsArray,
		},
	}

	responseBytes := util.MustToJSONBytes(response)
	result.Payload = responseBytes
	result.Status = 200

	return result, nil
}

func (handler *SQLiteORM) DeleteByQuery(ctx *api.Context, qb *api.QueryBuilder) (*api.DeleteByQueryResponse, error) {
	if qb == nil {
		return nil, errors.New("query builder is required for delete by query")
	}

	var indexName string
	if ctx != nil {
		if indices := api.GetIndices(ctx); len(indices) > 0 {
			indexName = indices[0]
		}
		if indexName == "" {
			model := api.GetModel(ctx)
			if model != nil {
				indexName = handler.GetIndexName(model)
			}
		}
	}
	if indexName == "" {
		return nil, errors.New("cannot resolve table name from context")
	}

	qb.Build()
	where, args := sqliteOrm.BuildWhereClause(qb)

	// Count before delete
	countSQL := fmt.Sprintf("SELECT COUNT(*) FROM [%s]", indexName)
	if where != "" {
		countSQL += " WHERE " + where
	}

	var total int64
	err := handler.DB.QueryRow(countSQL, args...).Scan(&total)
	if err != nil {
		return nil, err
	}

	// Delete
	sqlStr := fmt.Sprintf("DELETE FROM [%s]", indexName)
	if where != "" {
		sqlStr += " WHERE " + where
	}

	if global.Env().IsDebug {
		log.Debug("sqlite DeleteByQuery: ", sqlStr, " args=", args)
	}

	result, err := handler.DB.Exec(sqlStr, args...)
	if err != nil {
		return nil, err
	}

	deleted, _ := result.RowsAffected()
	return &api.DeleteByQueryResponse{
		Deleted: deleted,
		Total:   total,
	}, nil
}

// getObjectID extracts the ID from an ORM object.
func getObjectID(o interface{}) string {
	// Try the elastic_meta tag approach first
	id := util.GetFieldValueByTagName(o, "elastic_meta", "_id")
	if id != "" {
		return id
	}

	// Fallback: try map keys
	rv := reflect.ValueOf(o)
	if !rv.IsValid() {
		return ""
	}
	if rv.Kind() == reflect.Ptr && !rv.IsNil() {
		rv = rv.Elem()
	}
	if rv.Kind() == reflect.Map && rv.Type().Key().Kind() == reflect.String {
		for _, key := range []string{"id", "_id"} {
			val := rv.MapIndex(reflect.ValueOf(key))
			if val.IsValid() && val.Kind() == reflect.Interface {
				val = val.Elem()
			}
			if val.IsValid() && val.Kind() == reflect.String {
				return val.String()
			}
		}
	}

	return ""
}

// buildLegacyWhere converts legacy orm.Cond slices to SQL WHERE clauses.
func buildLegacyWhere(conds []*api.Cond) ([]string, []interface{}) {
	var clauses []string
	var args []interface{}

	for _, c := range conds {
		jsonPath := fmt.Sprintf("json_extract(raw, '$.%s')", c.Field)
		switch c.QueryType {
		case api.Match:
			if c.BoolType == api.MustNot {
				clauses = append(clauses, fmt.Sprintf("%s != ?", jsonPath))
			} else {
				clauses = append(clauses, fmt.Sprintf("%s = ?", jsonPath))
			}
			args = append(args, c.Value)
		case api.PrefixQueryType:
			clauses = append(clauses, fmt.Sprintf("%s LIKE ?", jsonPath))
			args = append(args, fmt.Sprintf("%v%%", c.Value))
		case api.Wildcard:
			clauses = append(clauses, fmt.Sprintf("%s LIKE ?", jsonPath))
			val := strings.ReplaceAll(fmt.Sprintf("%v", c.Value), "*", "%")
			val = strings.ReplaceAll(val, "?", "_")
			args = append(args, val)
		case api.QueryStringType:
			clauses = append(clauses, fmt.Sprintf("%s LIKE ?", jsonPath))
			args = append(args, fmt.Sprintf("%%%v%%", c.Value))
		case api.RangeGt:
			clauses = append(clauses, fmt.Sprintf("%s > ?", jsonPath))
			args = append(args, c.Value)
		case api.RangeGte:
			clauses = append(clauses, fmt.Sprintf("%s >= ?", jsonPath))
			args = append(args, c.Value)
		case api.RangeLt:
			clauses = append(clauses, fmt.Sprintf("%s < ?", jsonPath))
			args = append(args, c.Value)
		case api.RangeLte:
			clauses = append(clauses, fmt.Sprintf("%s <= ?", jsonPath))
			args = append(args, c.Value)
		case api.Terms:
			if vals, ok := c.Value.([]interface{}); ok && len(vals) > 0 {
				placeholders := strings.Repeat("?,", len(vals))
				placeholders = placeholders[:len(placeholders)-1]
				clauses = append(clauses, fmt.Sprintf("%s IN (%s)", jsonPath, placeholders))
				args = append(args, vals...)
			}
		case api.StringTerms:
			if vals, ok := c.Value.([]string); ok && len(vals) > 0 {
				placeholders := strings.Repeat("?,", len(vals))
				placeholders = placeholders[:len(placeholders)-1]
				clauses = append(clauses, fmt.Sprintf("%s IN (%s)", jsonPath, placeholders))
				for _, v := range vals {
					args = append(args, v)
				}
			}
		default:
			clauses = append(clauses, fmt.Sprintf("%s = ?", jsonPath))
			args = append(args, c.Value)
		}
	}
	return clauses, args
}
