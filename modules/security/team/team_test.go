/* Copyright © INFINI Ltd. All rights reserved.
 * Web: https://infinilabs.com
 * Email: hello#infini.ltd */

package team

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
	esorm "infini.sh/framework/modules/elastic/orm"
)

// TestMemberMatches keeps account IDs and logins in separate namespaces.
func TestMemberMatches(t *testing.T) {
	for _, tc := range []struct {
		member    MemberId
		id, login string
		match     bool
	}{
		{MemberId{MemberKindUserID, "alice"}, "alice", "bob", true},
		{MemberId{MemberKindLogin, "bob"}, "alice", "bob", true},
		{MemberId{MemberKindUserID, "bob"}, "alice", "bob", false},
		{MemberId{MemberKindLogin, "alice"}, "alice", "bob", false},
		{MemberId{MemberKindLogin, "display-name"}, "alice", "bob", false},
		{MemberId{MemberKindLogin, ""}, "", "", false},
		{MemberId{"name", "alice"}, "alice", "alice", false},
	} {
		require.Equal(t, tc.match, memberMatches(tc.member, tc.id, tc.login), "%+v", tc)
	}
}

// TestNormalizeMembers rejects invalid identities and preserves distinct reference kinds.
func TestNormalizeMembers(t *testing.T) {
	members, err := normalizeMembers([]MemberId{{MemberKindUserID, " alice "}, {MemberKindUserID, "alice"}, {MemberKindLogin, "alice"}})
	require.NoError(t, err)
	require.Equal(t, []MemberId{{MemberKindUserID, "alice"}, {MemberKindLogin, "alice"}}, members)
	for _, member := range []MemberId{{"name", "alice"}, {"", "alice"}, {MemberKindLogin, " "}} {
		_, err := normalizeMembers([]MemberId{{MemberKindUserID, "valid"}, member})
		require.Error(t, err)
	}
	members, err = normalizeMembers(nil)
	require.NoError(t, err)
	require.Empty(t, members)
}

// TestInvalidTeamMembers rejects the whole request before accessing storage.
func TestInvalidTeamMembers(t *testing.T) {
	handler := &APIHandler{}
	for _, payload := range []string{`{"name":"team","members":["alice"]}`, `{"name":"team","members":[{"kind":"name","value":"alice"}]}`, `{"name":"team","members":[{"kind":"login","value":" "}]}`} {
		for _, method := range []string{http.MethodPost, http.MethodPut} {
			req := httptest.NewRequest(method, "/security/teams/team", strings.NewReader(payload))
			response := httptest.NewRecorder()
			if method == http.MethodPost {
				handler.create(response, req, nil)
			} else {
				handler.update(response, req, httprouter.Params{{Key: "id", Value: "team"}})
			}
			require.Equal(t, http.StatusBadRequest, response.Code)
		}
	}
}

type teamSearchBackend struct {
	orm.ORM
	search func(*orm.Context, *orm.QueryBuilder) (*orm.SearchResult, error)
}

func (b *teamSearchBackend) SearchV2(ctx *orm.Context, qb *orm.QueryBuilder) (*orm.SearchResult, error) {
	return b.search(ctx, qb)
}

// TestTeamPagination ignores approximate totals and uses unfiltered page lengths.
func TestTeamPagination(t *testing.T) {
	backend := &teamSearchBackend{}
	orm.Register(t.TempDir(), backend)
	for _, list := range []bool{false, true} {
		calls := 0
		backend.search = func(ctx *orm.Context, qb *orm.QueryBuilder) (*orm.SearchResult, error) {
			calls++
			require.Equal(t, (calls-1)*500, qb.FromVal())
			if !list {
				conditions := qb.Root().ShouldClauses
				require.Len(t, conditions, 2)
				for _, condition := range conditions {
					require.Equal(t, orm.QueryNested, condition.Operator)
					require.Equal(t, "members", condition.Parameters.MustGetString("path"))
					require.Len(t, condition.Value.(*orm.Clause).MustClauses, 2)
				}
			}
			count := 500
			if calls == 2 {
				count = 1
			}
			hits := make([]util.MapStr, count)
			for i := range hits {
				members := []MemberId{{MemberKindUserID, "other"}}
				if calls == 2 {
					members = []MemberId{{MemberKindLogin, "alice"}}
				}
				hits[i] = util.MapStr{"_id": fmt.Sprintf("%d-%d", calls, i), "_source": util.MapStr{"members": members}}
			}
			return &orm.SearchResult{Status: 200, Payload: util.MustToJSONBytes(util.MapStr{"hits": util.MapStr{
				"total": util.MapStr{"value": 0, "relation": "gte"}, "hits": hits}})}, nil
		}
		if list {
			teams, err := List(orm.NewContext().DirectReadAccess())
			require.NoError(t, err)
			require.Len(t, teams, 501)
		} else {
			require.Equal(t, []string{"2-0"}, TeamIDsForUser("user-id", "alice"))
		}
		require.Equal(t, 2, calls)
	}
}

// TestMemberMapping retains exact keyword values inside the nested member mapping.
func TestMemberMapping(t *testing.T) {
	var members *util.Annotation
	for _, field := range util.GetTagsByTagName(Team{}, "elastic_mapping") {
		if field.Field == "Members" {
			members = &field
			break
		}
	}
	require.NotNil(t, members)
	require.Contains(t, members.Tag, "type: nested")
	require.Len(t, members.Annotation, 2)
	require.Equal(t, "kind: {type: keyword}", members.Annotation[0].Tag)
	require.Equal(t, "value: {type: keyword}", members.Annotation[1].Tag)
}

// TestMembershipQuery binds each kind and value within the same nested member.
func TestMembershipQuery(t *testing.T) {
	backend := &teamSearchBackend{}
	orm.Register(t.TempDir(), backend)
	for _, identity := range []struct {
		id, login  string
		conditions string
	}{
		{"ID", "", `{"nested":{"path":"members","query":{"bool":{"must":[{"term":{"members.kind":{"value":"user_id"}}},{"term":{"members.value":{"value":"ID"}}}]}}}}`},
		{"", "Login", `{"nested":{"path":"members","query":{"bool":{"must":[{"term":{"members.kind":{"value":"login"}}},{"term":{"members.value":{"value":"Login"}}}]}}}}`},
	} {
		calls := 0
		backend.search = func(_ *orm.Context, qb *orm.QueryBuilder) (*orm.SearchResult, error) {
			calls++
			query := esorm.BuildQueryDSL(qb)["query"]
			require.JSONEq(t, identity.conditions, util.MustToJSON(query))
			return &orm.SearchResult{Status: 200, Payload: []byte(`{"hits":{"hits":[]}}`)}, nil
		}
		require.Empty(t, TeamIDsForUser(identity.id, identity.login))
		require.Equal(t, 1, calls)
	}
	backend.search = func(*orm.Context, *orm.QueryBuilder) (*orm.SearchResult, error) {
		t.Fatal("empty identity must not query storage")
		return nil, nil
	}
	require.Empty(t, TeamIDsForUser("", ""))
}
