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
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"infini.sh/framework/core/orm"
)

// TestNestedMembers matches kind and value in one object and rejects unsupported clauses.
func TestNestedMembers(t *testing.T) {
	backend, cleanup := setupTestDB(t)
	defer cleanup()
	for id, members := range map[string]string{
		"match":     `[{"kind":"user_id","value":"alice"}]`,
		"cross":     `[{"kind":"user_id","value":"bob"},{"kind":"login","value":"alice"}]`,
		"mixed":     `["alice",null,1,{"kind":"user_id","value":"alice"}]`,
		"primitive": `["alice",null,1]`, "empty": `[]`, "null": `null`, "object": `{"kind":"user_id","value":"alice"}`,
		"escaped": `[{"kind":"user_id","value":"alice' OR 1=1 --"}]`,
	} {
		_, err := backend.DB.Exec(fmt.Sprintf("INSERT INTO [%s] (id,raw) VALUES (?,?)", backend.GetIndexName(TestItem{})), id, `{"id":"`+id+`","members":`+members+`}`)
		require.NoError(t, err)
	}
	term := orm.MustQuery(orm.TermQuery("members.kind", "user_id"), orm.TermQuery("members.value", "alice"))
	filters := orm.BooleanQuery()
	filters.FilterClauses = []*orm.Clause{orm.TermQuery("members.kind", "user_id"), orm.MustQuery(orm.TermsQuery("members.value", []string{"alice"}))}
	mixedUnsupported := orm.MustQuery(term, orm.MatchQuery("members.value", "alice"))
	minShould := orm.MustQuery(term).Parameter("minimum_should_match", 0)
	for _, tc := range []struct {
		name, path string
		query      *orm.Clause
		ids        []string
	}{
		{"same object", "members", term, []string{"match", "mixed"}},
		{"recursive filter and terms", "members", filters, []string{"match", "mixed"}},
		{"escaped value", "members", orm.MustQuery(orm.TermQuery("members.kind", "user_id"), orm.TermQuery("members.value", "alice' OR 1=1 --")), []string{"escaped"}},
		{"empty terms", "members", orm.TermsQuery("members.value", []string{}), nil},
		{"unsupported child", "members", mixedUnsupported, nil},
		{"must not", "members", orm.MustNotQuery(term), nil},
		{"should", "members", orm.ShouldQuery(term), nil},
		{"minimum should match", "members", minShould, nil},
		{"empty bool", "members", orm.BooleanQuery(), nil},
		{"nested within nested", "members", orm.NestedQuery("members", term), nil},
		{"outside field", "members", orm.TermQuery("id", "match"), nil},
		{"invalid field", "members", orm.TermQuery("members.value' OR 1=1 --", "alice"), nil},
		{"array index", "members[0]", term, nil},
		{"invalid path", "members'", term, nil},
		{"empty path", "", term, nil},
		{"invalid prefix", "members", orm.TermQuery("members2.value", "alice"), nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result, err := backend.SearchV2(orm.WithModel(orm.NewContext(), &TestItem{}), orm.NewQuery().Filter(orm.NestedQuery(tc.path, tc.query)).Size(100))
			require.NoError(t, err)
			ids := requireSearchResultIDs(t, result)
			require.ElementsMatch(t, tc.ids, ids)
		})
	}
}
