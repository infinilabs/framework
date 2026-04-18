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
