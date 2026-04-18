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

package orm

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"infini.sh/framework/core/orm"
)

func TestBuildWhereClause_NilQueryBuilder(t *testing.T) {
	where, args := BuildWhereClause(nil)
	assert.Equal(t, "", where)
	assert.Nil(t, args)
}

func TestBuildWhereClause_EmptyQueryBuilder(t *testing.T) {
	qb := orm.NewQuery()
	qb.Build()
	where, args := BuildWhereClause(qb)
	assert.Equal(t, "", where)
	assert.Nil(t, args)
}

func TestBuildWhereClause_SingleTermFilter(t *testing.T) {
	qb := orm.NewQuery()
	qb.Filter(orm.TermQuery("status", "active"))
	qb.Build()
	where, args := BuildWhereClause(qb)
	assert.Equal(t, "json_extract(raw, '$.status') = ?", where)
	assert.Equal(t, []interface{}{"active"}, args)
}

func TestBuildWhereClause_MultipleFilters(t *testing.T) {
	qb := orm.NewQuery()
	qb.Filter(orm.TermQuery("status", "active"))
	qb.Filter(orm.TermQuery("type", "node"))
	qb.Build()
	where, args := BuildWhereClause(qb)
	assert.Contains(t, where, "json_extract(raw, '$.status') = ?")
	assert.Contains(t, where, "json_extract(raw, '$.type') = ?")
	assert.Equal(t, 2, len(args))
}

func TestBuildWhereClause_RangeQuery(t *testing.T) {
	qb := orm.NewQuery()
	qb.Filter(orm.Range("age").Gte(18))
	qb.Filter(orm.Range("age").Lt(65))
	qb.Build()
	where, args := BuildWhereClause(qb)
	assert.Contains(t, where, "json_extract(raw, '$.age') >= ?")
	assert.Contains(t, where, "json_extract(raw, '$.age') < ?")
	assert.Equal(t, 2, len(args))
}

func TestBuildWhereClause_PrefixQuery(t *testing.T) {
	qb := orm.NewQuery()
	qb.Filter(orm.PrefixQuery("name", "john"))
	qb.Build()
	where, args := BuildWhereClause(qb)
	assert.Equal(t, "json_extract(raw, '$.name') LIKE ?", where)
	assert.Equal(t, []interface{}{"john%"}, args)
}

func TestBuildWhereClause_WildcardQuery(t *testing.T) {
	qb := orm.NewQuery()
	qb.Filter(orm.WildcardQuery("name", "j*n"))
	qb.Build()
	where, args := BuildWhereClause(qb)
	assert.Equal(t, "json_extract(raw, '$.name') LIKE ?", where)
	assert.Equal(t, []interface{}{"j%n"}, args)
}

func TestBuildWhereClause_ExistsQuery(t *testing.T) {
	qb := orm.NewQuery()
	qb.Filter(orm.ExistsQuery("email"))
	qb.Build()
	where, args := BuildWhereClause(qb)
	assert.Equal(t, "json_extract(raw, '$.email') IS NOT NULL", where)
	assert.Nil(t, args)
}

func TestBuildWhereClause_MustNotClause(t *testing.T) {
	qb := orm.NewQuery()
	qb.Not(orm.TermQuery("status", "deleted"))
	qb.Build()
	where, args := BuildWhereClause(qb)
	assert.Contains(t, where, "NOT")
	assert.Contains(t, where, "json_extract(raw, '$.status') = ?")
	assert.Equal(t, []interface{}{"deleted"}, args)
}

func TestBuildWhereClause_ShouldClauses(t *testing.T) {
	qb := orm.NewQuery()
	qb.Should(
		orm.TermQuery("status", "active"),
		orm.TermQuery("status", "pending"),
	)
	qb.Build()
	where, args := BuildWhereClause(qb)
	assert.Contains(t, where, "OR")
	assert.Equal(t, 2, len(args))
}

func TestBuildWhereClause_TermsQuery(t *testing.T) {
	qb := orm.NewQuery()
	qb.Filter(orm.TermsQuery("status", []string{"active", "pending"}))
	qb.Build()
	where, args := BuildWhereClause(qb)
	assert.Contains(t, where, "IN")
	assert.Equal(t, 2, len(args))
}

func TestBuildWhereClause_MatchQuery(t *testing.T) {
	qb := orm.NewQuery()
	qb.Must(orm.MatchQuery("title", "hello"))
	qb.Build()
	where, args := BuildWhereClause(qb)
	assert.Equal(t, "json_extract(raw, '$.title') = ?", where)
	assert.Equal(t, []interface{}{"hello"}, args)
}

// Tests below verify that the SQLite query builder generates correct SQL
// for the complex nested query patterns used by the security search
// operation hook (RegisterSearchOperationHook), including nested boolean
// clauses, mixed should/must_not compound queries, and dotted field paths.

func TestBuildWhereClause_MustWrappingShouldQuery(t *testing.T) {
	// Mirrors: qb.Must(bq) where bq = ShouldQuery() with multiple clauses
	qb := orm.NewQuery()

	bq := orm.ShouldQuery()
	bq.ShouldClauses = append(bq.ShouldClauses, orm.TermQuery("_system.owner_id", "user123"))
	bq.ShouldClauses = append(bq.ShouldClauses, orm.TermsQuery("id", []string{"a", "b"}))
	bq.Parameter("minimum_should_match", 1)

	qb.Must(bq)
	qb.Build()

	where, args := BuildWhereClause(qb)
	assert.Contains(t, where, "OR")
	assert.Contains(t, where, "json_extract(raw, '$._system.owner_id') = ?")
	assert.Contains(t, where, "IN")
	assert.Equal(t, 3, len(args)) // "user123", "a", "b"
}

func TestBuildWhereClause_OwnerOnlyNoSharing(t *testing.T) {
	// When sharing is disabled, only owner_id filter is applied
	qb := orm.NewQuery()

	bq := orm.ShouldQuery()
	bq.ShouldClauses = append(bq.ShouldClauses, orm.TermQuery("_system.owner_id", "user1"))

	qb.Must(bq)
	qb.Build()

	where, args := BuildWhereClause(qb)
	assert.Equal(t, "json_extract(raw, '$._system.owner_id') = ?", where)
	assert.Equal(t, []interface{}{"user1"}, args)
}

func TestBuildWhereClause_ShouldWithSharedIDs(t *testing.T) {
	// Sharing enabled: owner OR shared resource IDs
	qb := orm.NewQuery()

	bq := orm.ShouldQuery()
	bq.ShouldClauses = append(bq.ShouldClauses, orm.TermsQuery("id", []string{"shared1", "shared2", "shared3"}))
	bq.ShouldClauses = append(bq.ShouldClauses, orm.TermQuery("_system.owner_id", "user1"))
	bq.Parameter("minimum_should_match", 1)

	qb.Must(bq)
	qb.Build()

	where, args := BuildWhereClause(qb)
	assert.Contains(t, where, "OR")
	assert.Contains(t, where, "IN")
	assert.Contains(t, where, "json_extract(raw, '$._system.owner_id') = ?")
	assert.Equal(t, 4, len(args)) // "shared1", "shared2", "shared3", "user1"
}

func TestBuildWhereClause_CategoryFilterWithOwner(t *testing.T) {
	// Category-level sharing: category filter field OR owner
	qb := orm.NewQuery()

	bq := orm.ShouldQuery()
	bq.ShouldClauses = append(bq.ShouldClauses, orm.TermQuery("datasource_id", "ds-123"))
	bq.ShouldClauses = append(bq.ShouldClauses, orm.TermQuery("_system.owner_id", "user1"))
	bq.Parameter("minimum_should_match", 1)

	qb.Must(bq)
	qb.Build()

	where, args := BuildWhereClause(qb)
	assert.Contains(t, where, "OR")
	assert.Contains(t, where, "json_extract(raw, '$.datasource_id') = ?")
	assert.Contains(t, where, "json_extract(raw, '$._system.owner_id') = ?")
	assert.Equal(t, 2, len(args))
}

func TestBuildWhereClause_NestedBoolInsideShould(t *testing.T) {
	// Mirrors folder-level access with denied items:
	// Should: (prefix_path AND NOT denied_ids) OR allowed_ids OR owner
	qb := orm.NewQuery()

	bq := orm.ShouldQuery()

	// Nested bool: must prefix AND must_not denied terms
	nested := orm.BooleanQuery()
	nested.MustClauses = append(nested.MustClauses, orm.PrefixQuery("_system.parent_path", "/data/folder1/"))
	nested.MustNotClauses = append(nested.MustNotClauses, orm.TermsQuery("id", []string{"denied1", "denied2"}))

	bq.ShouldClauses = append(bq.ShouldClauses, nested)
	bq.ShouldClauses = append(bq.ShouldClauses, orm.TermsQuery("id", []string{"allowed1"}))
	bq.ShouldClauses = append(bq.ShouldClauses, orm.TermQuery("_system.owner_id", "user1"))
	bq.Parameter("minimum_should_match", 1)

	qb.Must(bq)
	qb.Build()

	where, args := BuildWhereClause(qb)
	assert.Contains(t, where, "LIKE ?")
	assert.Contains(t, where, "NOT")
	assert.Contains(t, where, "OR")
	assert.Contains(t, where, "json_extract(raw, '$._system.parent_path') LIKE ?")
	assert.Contains(t, where, "json_extract(raw, '$._system.owner_id') = ?")
	assert.NotEmpty(t, args)
}

func TestBuildWhereClause_ShouldWithMustNot_FolderDenyRules(t *testing.T) {
	// Mirrors: bq with ShouldClauses + MustNotClauses (deny rules)
	qb := orm.NewQuery()

	bq := orm.ShouldQuery()
	// Allow rules
	bq.ShouldClauses = append(bq.ShouldClauses, orm.TermsQuery("id", []string{"a1", "a2"}))
	bq.ShouldClauses = append(bq.ShouldClauses, orm.PrefixQuery("_system.parent_path", "/shared/"))
	bq.ShouldClauses = append(bq.ShouldClauses, orm.TermQuery("_system.owner_id", "user1"))

	// Deny individual IDs
	bq.MustNotClauses = append(bq.MustNotClauses, orm.TermsQuery("id", []string{"d1", "d2"}))

	// Deny folder path (compound must_not: prefix path excluding allowed IDs)
	folderExclude := orm.BooleanQuery()
	folderExclude.MustClauses = append(folderExclude.MustClauses, orm.PrefixQuery("_system.parent_path", "/denied/"))
	folderExclude.MustNotClauses = append(folderExclude.MustNotClauses, orm.TermsQuery("id", []string{"a1", "a2"}))
	bq.MustNotClauses = append(bq.MustNotClauses, folderExclude)

	bq.Parameter("minimum_should_match", 1)
	qb.Must(bq)
	qb.Build()

	where, args := BuildWhereClause(qb)
	assert.Contains(t, where, "OR")
	assert.Contains(t, where, "NOT")
	assert.NotEmpty(t, args)

	// Verify the deny rules generate NOT clauses
	// The folder exclude should produce: NOT (prefix_path LIKE ? AND NOT (id IN (?,...)))
	assert.Contains(t, where, "json_extract(raw, '$._system.parent_path') LIKE ?")
}

func TestBuildWhereClause_DottedFieldPaths(t *testing.T) {
	// Verify json_extract handles nested/dotted field paths correctly
	qb := orm.NewQuery()
	qb.Filter(orm.TermQuery("_system.owner_id", "user1"))
	qb.Build()
	where, args := BuildWhereClause(qb)
	assert.Equal(t, "json_extract(raw, '$._system.owner_id') = ?", where)
	assert.Equal(t, []interface{}{"user1"}, args)
}

func TestBuildWhereClause_FilterWithMustQuery(t *testing.T) {
	// Mirrors: qb.Filter(orm.MustQuery(orm.TermQuery(...)))
	qb := orm.NewQuery()
	qb.Filter(orm.MustQuery(orm.TermQuery("_system.owner_id", "user1")))
	qb.Build()
	where, args := BuildWhereClause(qb)
	assert.Contains(t, where, "json_extract(raw, '$._system.owner_id') = ?")
	assert.Equal(t, []interface{}{"user1"}, args)
}

func TestBuildWhereClause_FullSearchHookSimulation(t *testing.T) {
	// Full simulation of the search hook query structure with sharing enabled:
	// - Category filter (OR)
	// - Inherited rules with folder access + denied items (OR)
	// - Allowed IDs (OR)
	// - Owner ID (OR)
	// All wrapped in Must
	qb := orm.NewQuery()

	bq := orm.ShouldQuery()

	// 1. Category filter
	bq.ShouldClauses = append(bq.ShouldClauses, orm.TermQuery("datasource_id", "ds-100"))

	// 2. Folder-level access with exclusions
	folderBool := orm.BooleanQuery()
	folderBool.MustClauses = append(folderBool.MustClauses, orm.PrefixQuery("_system.parent_path", "/workspace/project1/"))
	folderBool.MustNotClauses = append(folderBool.MustNotClauses, orm.TermsQuery("id", []string{"blocked-doc-1"}))
	bq.ShouldClauses = append(bq.ShouldClauses, folderBool)

	// 3. Explicitly allowed IDs
	bq.ShouldClauses = append(bq.ShouldClauses, orm.TermsQuery("id", []string{"doc-a", "doc-b", "doc-c"}))

	// 4. Owner filter (always present)
	bq.ShouldClauses = append(bq.ShouldClauses, orm.TermQuery("_system.owner_id", "user42"))

	bq.Parameter("minimum_should_match", 1)
	qb.Must(bq)
	qb.Build()

	where, args := BuildWhereClause(qb)
	assert.NotEmpty(t, where)
	assert.NotEmpty(t, args)

	// Should contain OR for the should clauses
	assert.Contains(t, where, "OR")

	// Should contain the category filter
	assert.Contains(t, where, "json_extract(raw, '$.datasource_id') = ?")

	// Should contain the prefix query for parent_path
	assert.Contains(t, where, "json_extract(raw, '$._system.parent_path') LIKE ?")

	// Should contain the NOT for excluded docs
	assert.Contains(t, where, "NOT")

	// Should contain IN for allowed IDs
	assert.Contains(t, where, "IN")

	// Should contain owner_id filter
	assert.Contains(t, where, "json_extract(raw, '$._system.owner_id') = ?")

	// Verify we have the right number of args:
	// "ds-100", "/workspace/project1/%", "blocked-doc-1", "doc-a", "doc-b", "doc-c", "user42"
	assert.Equal(t, 7, len(args))
}

func TestBuildWhereClause_MultipleFolderAllowAndDenyPaths(t *testing.T) {
	// Simulates the no-parent-path branch with multiple folder allow/deny rules
	qb := orm.NewQuery()

	bq := orm.ShouldQuery()

	// Allowed IDs
	bq.ShouldClauses = append(bq.ShouldClauses, orm.TermsQuery("id", []string{"doc1", "doc2"}))
	// Allowed folder paths
	bq.ShouldClauses = append(bq.ShouldClauses, orm.PrefixQuery("_system.parent_path", "/allowed/folder1/"))
	bq.ShouldClauses = append(bq.ShouldClauses, orm.PrefixQuery("_system.parent_path", "/allowed/folder2/"))
	// Owner
	bq.ShouldClauses = append(bq.ShouldClauses, orm.TermQuery("_system.owner_id", "user1"))

	// Deny individual IDs
	bq.MustNotClauses = append(bq.MustNotClauses, orm.TermsQuery("id", []string{"denied1"}))

	// Deny folder paths (compound: prefix AND NOT allowed_ids)
	for _, deniedPath := range []string{"/denied/folder1/", "/denied/folder2/"} {
		folderExclude := orm.BooleanQuery()
		folderExclude.MustClauses = append(folderExclude.MustClauses,
			orm.PrefixQuery("_system.parent_path", deniedPath))
		folderExclude.MustNotClauses = append(folderExclude.MustNotClauses,
			orm.TermsQuery("id", []string{"doc1", "doc2"}))
		bq.MustNotClauses = append(bq.MustNotClauses, folderExclude)
	}

	bq.Parameter("minimum_should_match", 1)
	qb.Must(bq)
	qb.Build()

	where, args := BuildWhereClause(qb)
	assert.NotEmpty(t, where)
	assert.Contains(t, where, "OR")
	assert.Contains(t, where, "NOT")
	assert.NotEmpty(t, args)
}

func TestBuildWhereClause_CategoryChildrenSharing(t *testing.T) {
	// Simulates category checking with children-based shared IDs
	qb := orm.NewQuery()

	bq := orm.ShouldQuery()
	// Shared via children
	bq.ShouldClauses = append(bq.ShouldClauses, orm.TermsQuery("id", []string{"cat1", "cat2"}))
	// Owner
	bq.ShouldClauses = append(bq.ShouldClauses, orm.TermQuery("_system.owner_id", "user1"))
	bq.Parameter("minimum_should_match", 1)

	qb.Must(bq)
	qb.Build()

	where, args := BuildWhereClause(qb)
	assert.Contains(t, where, "OR")
	assert.Contains(t, where, "IN")
	assert.Contains(t, where, "json_extract(raw, '$._system.owner_id') = ?")
	assert.Equal(t, 3, len(args)) // "cat1", "cat2", "user1"
}

func TestBuildWhereClause_EmptyShouldStaysValid(t *testing.T) {
	// Edge case: ShouldQuery with no clauses appended
	qb := orm.NewQuery()
	bq := orm.ShouldQuery()
	qb.Must(bq)
	qb.Build()

	where, args := BuildWhereClause(qb)
	// Empty boolean should be simplified away
	assert.Equal(t, "", where)
	assert.Nil(t, args)
}
