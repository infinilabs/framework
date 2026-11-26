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

package native

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/security"
)

// TestConcurrentAccountCreation permits one winner without replacing its password or roles.
func TestConcurrentAccountCreation(t *testing.T) {
	for _, scenario := range []struct {
		httpCount int
		force     bool
	}{{0, false}, {1, false}, {2, false}, {0, true}} {
		httpCount := scenario.httpCount
		t.Run(fmt.Sprintf("HTTP writers %d force %v", httpCount, scenario.force), func(t *testing.T) {
			backend := setupAccountStore(t)
			arrived := make(chan struct{}, 2)
			release := make(chan struct{}, 2)
			defer close(release)
			wrapper := &accountTestORM{ORM: backend}
			wrapper.search = func(ctx *orm.Context, qb *orm.QueryBuilder) (*orm.SearchResult, error) {
				result, err := backend.SearchV2(ctx, qb)
				arrived <- struct{}{}
				<-release
				return result, err
			}
			orm.Register(t.TempDir(), wrapper)
			type outcome struct {
				index, status int
				account       *security.UserAccount
				err           error
				body          string
			}
			results := make(chan outcome, 2)
			for i := 0; i < 2; i++ {
				go func(index int) {
					result := outcome{index: index}
					if index < httpCount {
						req := httptest.NewRequest(http.MethodPost, "/security/user/", strings.NewReader(`{"name":"http","email":"race@example.com","roles":["reader"]}`))
						req = req.WithContext(security.AddUserToContext(req.Context(), &security.UserSessionInfo{UserID: "admin", Roles: []string{security.RoleAdmin}}))
						response := httptest.NewRecorder()
						CreateUser(response, req, nil)
						result.status = response.Code
						result.body = response.Body.String()
						if result.status == http.StatusOK {
							result.account = &security.UserAccount{}
							result.err = json.Unmarshal(response.Body.Bytes(), result.account)
						}
					} else {
						result.account, result.err = (&SecurityBackendProvider{}).CreateUser("internal", "race@example.com", fmt.Sprintf("Password!Number%d", index), scenario.force)
					}
					results <- result
				}(i)
			}
			for i := 0; i < 2; i++ {
				select {
				case <-arrived:
				case <-time.After(5 * time.Second):
					t.Fatal("writers did not reach the lookup barrier")
				}
			}
			release <- struct{}{}
			release <- struct{}{}
			outcomes := make([]outcome, 0, 2)
			for i := 0; i < 2; i++ {
				select {
				case result := <-results:
					outcomes = append(outcomes, result)
				case <-time.After(5 * time.Second):
					t.Fatal("writer did not finish")
				}
			}
			winners := 0
			for _, result := range outcomes {
				if result.account != nil {
					winners++
					require.NoError(t, result.err)
					stored := &security.UserAccount{}
					stored.ID = result.account.ID
					found, err := backend.Get(orm.NewContext(), stored)
					require.NoError(t, err)
					require.True(t, found)
					password := result.account.Password
					if result.index >= httpCount {
						password = fmt.Sprintf("Password!Number%d", result.index)
					}
					require.NoError(t, bcrypt.CompareHashAndPassword([]byte(stored.Password), []byte(password)))
					require.Equal(t, result.account.Roles, stored.Roles)
				} else if result.index < httpCount {
					require.Equal(t, http.StatusConflict, result.status, result.body)
					require.NotContains(t, result.body, `"password"`)
				} else {
					require.ErrorIs(t, result.err, security.ErrIdentityConflict)
				}
			}
			require.Equal(t, 1, winners)
			var count int
			require.NoError(t, backend.DB.QueryRow("SELECT COUNT(*) FROM [app-users]").Scan(&count))
			require.Equal(t, 1, count)
		})
	}
}

// TestFailedAccountCreation returns failure without a password when storage reports an error.
func TestFailedAccountCreation(t *testing.T) {
	for _, persist := range []bool{false, true} {
		t.Run(fmt.Sprintf("persisted %v", persist), func(t *testing.T) {
			backend := setupAccountStore(t)
			wrapper := &accountTestORM{ORM: backend}
			wrapper.create = func(ctx *orm.Context, account interface{}) error {
				if persist {
					if err := backend.Create(ctx, account); err != nil {
						return err
					}
				}
				return fmt.Errorf("write outcome unknown")
			}
			orm.Register(t.TempDir(), wrapper)
			req := httptest.NewRequest(http.MethodPost, "/security/user/", strings.NewReader(`{"name":"alice","email":"alice@example.com"}`))
			req = req.WithContext(security.AddUserToContext(req.Context(), &security.UserSessionInfo{UserID: "admin", Roles: []string{security.RoleAdmin}}))
			response := httptest.NewRecorder()
			CreateUser(response, req, nil)
			status := http.StatusInternalServerError
			if persist {
				status = http.StatusConflict
			}
			require.Equal(t, status, response.Code)
			require.NotContains(t, response.Body.String(), `"password"`)
		})
	}
}

// TestCreateRejectsExistingLogin returns a conflict and preserves the stored account.
func TestCreateRejectsExistingLogin(t *testing.T) {
	backend := setupAccountStore(t)
	account := &security.UserAccount{Name: "alice", Email: "alice@example.com"}
	account.ID = getUIDByEmail(account.Email)
	require.NoError(t, backend.Create(orm.NewContext(), account))
	req := httptest.NewRequest(http.MethodPost, "/security/user/", strings.NewReader(`{"name":"alice","email":"alice@example.com"}`))
	response := httptest.NewRecorder()
	CreateUser(response, req, nil)
	require.Equal(t, http.StatusConflict, response.Code)
	require.NotContains(t, response.Body.String(), `"password"`)
	var count int
	require.NoError(t, backend.DB.QueryRow("SELECT COUNT(*) FROM [app-users]").Scan(&count))
	require.Equal(t, 1, count)
}

// TestForcedAccountUpdate preserves identity and cannot recreate a concurrently deleted user.
func TestForcedAccountUpdate(t *testing.T) {
	for _, deleted := range []bool{false, true} {
		t.Run(fmt.Sprintf("deleted %v", deleted), func(t *testing.T) {
			backend := setupAccountStore(t)
			original := &security.UserAccount{Name: "original", Email: "alice@example.com", Roles: []string{"reader"}}
			original.ID = "legacy-id"
			require.NoError(t, orm.Create(orm.NewContext().DirectAccess(), original))
			wrapper := &accountTestORM{ORM: backend}
			wrapper.update = func(ctx *orm.Context, value interface{}) error {
				if deleted {
					if err := backend.Delete(ctx, value); err != nil {
						return err
					}
				}
				return backend.Update(ctx, value)
			}
			orm.Register(t.TempDir(), wrapper)
			const password = "Updated!Password123"
			account, err := (&SecurityBackendProvider{}).CreateUser("updated", original.Email, password, true)
			if deleted {
				require.Error(t, err)
				require.Nil(t, account)
				found, _, err := (&SecurityBackendProvider{}).GetUserByID(original.ID)
				require.NoError(t, err)
				require.False(t, found)
			} else {
				require.NoError(t, err)
				require.Equal(t, original.ID, account.ID)
				found, stored, err := (&SecurityBackendProvider{}).GetUserByID(original.ID)
				require.NoError(t, err)
				require.True(t, found)
				require.Equal(t, "updated", stored.Name)
				require.Equal(t, original.Email, stored.Email)
				require.Equal(t, []string{security.RoleAdmin}, stored.Roles)
				require.NoError(t, bcrypt.CompareHashAndPassword([]byte(stored.Password), []byte(password)))
			}
		})
	}
}
