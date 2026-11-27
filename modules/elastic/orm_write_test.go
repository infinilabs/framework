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

package elastic

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/security"
	"infini.sh/framework/modules/elastic/common"
)

// TestAccountCreateRequest verifies create-only requests and 409 handling.
func TestAccountCreateRequest(t *testing.T) {
	elastic.ResetClientCacheForTest()
	elastic.RegisterClientProvider(common.InitClientWithConfig)
	for _, version := range []string{"1.0.0", "5.0.0", "7.0.0", "8.0.0"} {
		t.Run(version, func(t *testing.T) {
			var mutex sync.Mutex
			var stored *security.UserAccount
			calls := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				mutex.Lock()
				defer mutex.Unlock()
				calls++
				if r.URL.Query().Get("op_type") != "create" && !strings.Contains(r.URL.Path, "/_create/") {
					t.Error("account creation used an operation that can overwrite a document")
				}
				var incoming security.UserAccount
				if err := json.NewDecoder(r.Body).Decode(&incoming); err != nil {
					t.Error(err)
					w.WriteHeader(500)
					return
				}
				w.Header().Set("Content-Type", "application/json")
				if stored != nil {
					w.WriteHeader(http.StatusConflict)
					_, _ = w.Write([]byte(`{"error":{"type":"version_conflict_engine_exception"},"status":409}`))
					return
				}
				stored = &incoming
				w.WriteHeader(http.StatusCreated)
				_, _ = w.Write([]byte(`{"result":"created"}`))
			}))
			defer server.Close()
			cfg := elastic.ElasticsearchConfig{Endpoint: server.URL, Version: version, Distribution: elastic.Elasticsearch, Enabled: true}
			cfg.ID = t.Name()
			client, err := elastic.GetOrCreateClient(cfg)
			require.NoError(t, err)
			backend := &ElasticORM{Client: client}
			account := &security.UserAccount{Name: "winner", Email: "alice@example.com", Password: "winner hash"}
			account.ID = "user"
			ctx := orm.NewContext()
			ctx.Refresh = orm.WaitForRefresh
			require.NoError(t, backend.Create(ctx, account))
			account.Password = "loser hash"
			require.Error(t, backend.Create(ctx, account))
			mutex.Lock()
			defer mutex.Unlock()
			require.Equal(t, 2, calls)
			require.Equal(t, "winner hash", stored.Password)
		})
	}
}
