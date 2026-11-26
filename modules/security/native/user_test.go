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
	"errors"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"infini.sh/framework/core/global"
	_ "infini.sh/framework/modules/security/orm_hooks"
	"infini.sh/framework/modules/sqlite"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/security"
	"infini.sh/framework/core/util"
)

type accountTestORM struct {
	orm.ORM
	search func(*orm.Context, *orm.QueryBuilder) (*orm.SearchResult, error)
	get    func(*orm.Context, interface{}) (bool, error)
	create func(*orm.Context, interface{}) error
	update func(*orm.Context, interface{}) error
}

func (b *accountTestORM) SearchV2(ctx *orm.Context, qb *orm.QueryBuilder) (*orm.SearchResult, error) {
	if b.search != nil {
		return b.search(ctx, qb)
	}
	return b.ORM.SearchV2(ctx, qb)
}
func (b *accountTestORM) Get(ctx *orm.Context, o interface{}) (bool, error) {
	if b.get != nil {
		return b.get(ctx, o)
	}
	return b.ORM.Get(ctx, o)
}
func (b *accountTestORM) Create(ctx *orm.Context, o interface{}) error {
	if b.create != nil {
		return b.create(ctx, o)
	}
	return b.ORM.Create(ctx, o)
}
func (b *accountTestORM) Update(ctx *orm.Context, o interface{}) error {
	if b.update != nil {
		return b.update(ctx, o)
	}
	return b.ORM.Update(ctx, o)
}

// TestNativeLookup covers account lookup results and backend errors.
func TestNativeLookup(t *testing.T) {
	backend := &accountTestORM{}
	orm.Register(t.TempDir(), backend)
	unavailable := errors.New("table unavailable")
	for _, tc := range []struct {
		status, count int
		err           error
		want          error
	}{
		{200, 0, nil, nil}, {200, 1, nil, nil},
		{404, 0, nil, unavailable}, {500, 0, nil, unavailable}, {0, 0, unavailable, unavailable},
	} {
		backend.search = func(_ *orm.Context, qb *orm.QueryBuilder) (*orm.SearchResult, error) {
			require.Equal(t, 1, qb.SizeVal())
			require.Len(t, qb.Root().FilterClauses, 1)
			require.Equal(t, "email", qb.Root().FilterClauses[0].Field)
			require.Equal(t, "alice", qb.Root().FilterClauses[0].Value)
			hits := []util.MapStr{}
			for i := 0; i < tc.count; i++ {
				hits = append(hits, util.MapStr{"_id": fmt.Sprint(i), "_source": util.MapStr{"email": "alice"}})
			}
			return &orm.SearchResult{Status: tc.status, Payload: util.MustToJSONBytes(util.MapStr{"hits": util.MapStr{"hits": hits}})}, tc.err
		}
		found, account, err := GetUserByLogin("alice")
		if tc.want != nil {
			require.Error(t, err)
			require.False(t, found)
			require.Nil(t, account)
		} else {
			require.NoError(t, err)
			require.Equal(t, tc.count == 1, found)
			if found {
				require.Equal(t, "alice", account.Email)
			} else {
				require.Nil(t, account)
			}
		}
	}
	for _, body := range []string{
		`{"_shards":{"failed":1},"hits":{"hits":[]}}`,
		`{"_shards":{"failed":1},"hits":{"hits":[{"_id":"id","_source":{"email":"alice"}}]}}`,
		`{"timed_out":true,"hits":{"hits":[]}}`,
		`{"error":{"type":"search_failed"},"hits":{"hits":[]}}`,
		`{}`, `not JSON`,
	} {
		backend.search = func(*orm.Context, *orm.QueryBuilder) (*orm.SearchResult, error) {
			return &orm.SearchResult{Status: 200, Payload: []byte(body)}, nil
		}
		found, account, err := GetUserByLogin("alice")
		require.Error(t, err)
		require.False(t, found)
		require.Nil(t, account)
	}
	for _, backendError := range []error{nil, fmt.Errorf("wrapped: %w", orm.ErrNotFound), unavailable} {
		backend.get = func(*orm.Context, interface{}) (bool, error) { return false, backendError }
		found, account, err := (&SecurityBackendProvider{}).GetUserByID("missing")
		require.False(t, found)
		require.Nil(t, account)
		if backendError == unavailable {
			require.ErrorIs(t, err, unavailable)
		} else {
			require.NoError(t, err)
		}
	}
}

// TestMain installs the application's ownership hooks for native write tests.
func TestMain(m *testing.M) {
	for _, setup := range global.GetFuncAfterSetup() {
		setup()
	}
	os.Exit(m.Run())
}

// Helper function to create isolated user storage for native account write tests.
func setupAccountStore(t *testing.T) *sqlite.SQLiteORM {
	t.Helper()
	backend := &sqlite.SQLiteORM{Config: sqlite.SQLiteConfig{Enabled: true, DBPath: filepath.Join(t.TempDir(), "accounts.db")}}
	require.NoError(t, backend.Open())
	t.Cleanup(func() { require.NoError(t, backend.Close()) })
	require.NoError(t, backend.RegisterSchemaWithName(security.UserAccount{}, "app-users"))
	orm.Register(backend.Config.DBPath, backend)
	return backend
}

// TestNativeAccountIDs derives distinct IDs from the supplied normalized emails.
func TestNativeAccountIDs(t *testing.T) {
	setupAccountStore(t)
	provider := &SecurityBackendProvider{}
	const password = "Correct!Password123"
	first, err := provider.CreateUser("first", " first@example.com ", password, false)
	require.NoError(t, err)
	second, err := provider.CreateUser("second", "second@example.com", password, false)
	require.NoError(t, err)
	require.Equal(t, getUIDByEmail("first@example.com"), first.ID)
	require.Equal(t, "first@example.com", first.Email)
	require.NotEqual(t, first.ID, second.ID)
	found, stored, err := provider.GetUserByID(first.ID)
	require.NoError(t, err)
	require.True(t, found)
	require.NoError(t, bcrypt.CompareHashAndPassword([]byte(stored.Password), []byte(password)))
}

// TestNativeEmailValidation rejects invalid input before any user can be persisted.
func TestNativeEmailValidation(t *testing.T) {
	backend := setupAccountStore(t)
	for _, email := range []string{"", " ", "alice"} {
		account, err := (&SecurityBackendProvider{}).CreateUser("name", email, "Correct!Password123", false)
		require.Error(t, err)
		require.Nil(t, account)
		request := httptest.NewRequest(http.MethodPost, "/security/user/", strings.NewReader(util.MustToJSON(util.MapStr{"name": "name", "email": email})))
		response := httptest.NewRecorder()
		CreateUser(response, request, nil)
		require.Equal(t, http.StatusBadRequest, response.Code)
	}
	var count int
	require.NoError(t, backend.DB.QueryRow("SELECT COUNT(*) FROM [app-users]").Scan(&count))
	require.Zero(t, count)
}
