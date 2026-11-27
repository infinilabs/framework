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

package team

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"infini.sh/framework/core/api"
	httprouter "infini.sh/framework/core/api/router"
	"infini.sh/framework/core/event"
	"infini.sh/framework/core/global"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/security"
	"infini.sh/framework/core/security/filters"
	_ "infini.sh/framework/modules/security/orm_hooks"
	"infini.sh/framework/modules/sqlite"
)

// TestMain installs the ownership hooks that application setup installs.
func TestMain(m *testing.M) {
	for _, setup := range global.GetFuncAfterSetup() {
		setup()
	}
	os.Exit(m.Run())
}

// TestTeamUpdateAuthorization checks route permissions with real ORM ownership hooks.
func TestTeamUpdateAuthorization(t *testing.T) {
	backend := &sqlite.SQLiteORM{Config: sqlite.SQLiteConfig{
		Enabled: true,
		DBPath:  filepath.Join(t.TempDir(), "teams.db"),
	}}
	require.NoError(t, backend.Open())
	t.Cleanup(func() { require.NoError(t, backend.Close()) })
	require.NoError(t, backend.RegisterSchemaWithName(Team{}, "security-team"))
	require.NoError(t, backend.RegisterSchemaWithName(event.Audit{}, "audit-logs"))
	orm.Register(backend.Config.DBPath, backend)

	securityConfig := &global.Env().SystemConfig.WebAppConfig.Security
	wasEnabled := securityConfig.Enabled
	securityConfig.Enabled = true
	t.Cleanup(func() { securityConfig.Enabled = wasEnabled })

	updatePermission := security.GetSimplePermission("generic", "security:team", security.Update)
	options := &api.HandlerOptions{}
	api.RequirePermission(updatePermission)(options)
	handler := &APIHandler{}
	filter := &filters.PermissionFilter{}
	update := filter.ApplyFilter(http.MethodPut, "/security/teams/:id", options, handler.update)

	for _, tc := range []struct {
		name        string
		userID      string
		roles       []string
		permissions []security.PermissionKey
		missing     bool
		status      int
	}{
		{name: "authorized nonowner", userID: "delegate", permissions: []security.PermissionKey{updatePermission}, status: http.StatusOK},
		{name: "admin", userID: "admin", roles: []string{security.RoleAdmin}, status: http.StatusOK},
		{name: "no update permission", userID: "delegate", status: http.StatusForbidden},
		{name: "anonymous", status: http.StatusUnauthorized},
		{name: "missing team", userID: "delegate", permissions: []security.PermissionKey{updatePermission}, missing: true, status: http.StatusNotFound},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := backend.DB.Exec("DELETE FROM [audit-logs]")
			require.NoError(t, err)
			original := Team{Name: "original", Description: "before", Members: []MemberId{{MemberKindLogin, "alice"}}}
			original.ID = strings.ReplaceAll(tc.name, " ", "-")
			original.SetOwnerID("owner")
			if !tc.missing {
				require.NoError(t, orm.Create(orm.NewContext().DirectAccess(), &original))
			}
			req := httptest.NewRequest(http.MethodPut, "/security/teams/"+original.ID, strings.NewReader(
				`{"id":"spoofed","name":"updated","description":"after","members":[{"kind":"login","value":"bob"}],"_system":{"owner_id":"spoofed"}}`))
			if tc.userID != "" {
				user := &security.UserSessionInfo{
					UserID:                 tc.userID,
					Roles:                  tc.roles,
					UserAssignedPermission: security.NewUserAssignedPermission(tc.permissions, nil),
				}
				req = req.WithContext(security.AddUserToContext(req.Context(), user))
			}
			response := httptest.NewRecorder()
			require.NotPanics(t, func() {
				update(response, req, httprouter.Params{{Key: "id", Value: original.ID}})
			})
			require.Equal(t, tc.status, response.Code, response.Body.String())

			stored := Team{}
			stored.ID = original.ID
			exists, err := backend.Get(orm.NewContext(), &stored)
			if tc.missing {
				require.ErrorIs(t, err, sqlite.ErrNotFound)
			} else {
				require.NoError(t, err)
			}
			require.Equal(t, !tc.missing, exists)
			if tc.status == http.StatusOK {
				require.Equal(t, original.ID, stored.ID)
				require.Equal(t, "owner", stored.GetOwnerID())
				require.Equal(t, "updated", stored.Name)
				require.Equal(t, "after", stored.Description)
				require.Equal(t, []MemberId{{MemberKindLogin, "bob"}}, stored.Members)
				var body struct {
					Result Team `json:"result"`
				}
				require.NoError(t, json.Unmarshal(response.Body.Bytes(), &body))
				require.Equal(t, stored.ID, body.Result.ID)
				require.Equal(t, stored.Members, body.Result.Members)

				if tc.userID == "delegate" {
					ctx := orm.NewContextWithParent(req.Context()).DirectReadAccess()
					stored.Name = "unauthorized ORM update"
					require.EqualError(t, orm.Update(ctx, &stored), "invalid data access")
					_, err = backend.Get(orm.NewContext(), &stored)
					require.NoError(t, err)
					require.Equal(t, "updated", stored.Name)
				}
			} else if exists {
				require.Equal(t, original.Name, stored.Name)
				require.Equal(t, original.Description, stored.Description)
				require.Equal(t, original.Members, stored.Members)
				require.Equal(t, "owner", stored.GetOwnerID())
			}

			// DirectAccess retains the existing policy of skipping ORM audit records.
			var auditCount int
			require.NoError(t, backend.DB.QueryRow("SELECT COUNT(*) FROM [audit-logs]").Scan(&auditCount))
			require.Zero(t, auditCount)
		})
	}
}

// TestStoredMembership matches typed identities through the SQLite ORM query path.
func TestStoredMembership(t *testing.T) {
	backend := &sqlite.SQLiteORM{Config: sqlite.SQLiteConfig{Enabled: true, DBPath: filepath.Join(t.TempDir(), "membership.db")}}
	require.NoError(t, backend.Open())
	t.Cleanup(func() { require.NoError(t, backend.Close()) })
	require.NoError(t, backend.RegisterSchemaWithName(Team{}, "security-team"))
	orm.Register(backend.Config.DBPath, backend)
	for id, members := range map[string][]MemberId{
		"by-id":         {{MemberKindUserID, "alice-id"}},
		"by-login":      {{MemberKindLogin, "alice"}},
		"wrong-kind":    {{MemberKindUserID, "alice"}, {MemberKindLogin, "alice-id"}},
		"cross-element": {{MemberKindUserID, "other"}, {MemberKindLogin, "alice-id"}},
	} {
		team := Team{Name: id, Members: members}
		team.ID = id
		require.NoError(t, orm.Create(orm.NewContext().DirectAccess(), &team))
	}
	require.ElementsMatch(t, []string{"by-id", "by-login"}, TeamIDsForUser("alice-id", "alice"))
	require.Empty(t, TeamIDsForUser("absent", "absent"))
}
