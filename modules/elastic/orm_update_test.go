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

// TestUpdateExistingRequests preserves strict Update, explicit Upsert, and direct client semantics.
func TestUpdateExistingRequests(t *testing.T) {
	elastic.ResetClientCacheForTest()
	elastic.RegisterClientProvider(common.InitClientWithConfig)
	for _, target := range []struct {
		version      string
		distribution string
	}{
		{"1.0.0", elastic.Elasticsearch}, {"5.0.0", elastic.Elasticsearch}, {"7.0.0", elastic.Elasticsearch},
		{"8.0.0", elastic.Elasticsearch}, {"9.0.0", elastic.Elasticsearch},
		{"1.0.0", elastic.Easysearch}, {"2.0.0", elastic.Opensearch},
	} {
		version := target.version
		for _, mode := range []string{"update", "deleted", "upsert", "direct", "noop"} {
			t.Run(target.distribution+"/"+version+"/"+mode, func(t *testing.T) {
				var mutex sync.Mutex
				exists := mode != "upsert" && mode != "direct"
				updates := 0
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					mutex.Lock()
					defer mutex.Unlock()
					w.Header().Set("Content-Type", "application/json")
					if r.Method == http.MethodGet {
						if !exists {
							w.WriteHeader(404)
							_, _ = w.Write([]byte(`{"_id":"user","found":false}`))
							return
						}
						_, _ = w.Write([]byte(`{"_id":"user","found":true,"_source":{"id":"user","name":"before","email":"alice@example.com"}}`))
						if mode == "deleted" {
							exists = false
						}
						return
					}
					updates++
					if !strings.Contains(r.URL.Path, "_update") {
						t.Errorf("unexpected update path %s", r.URL.Path)
					}
					if target.distribution == elastic.Elasticsearch && (version == "1.0.0" || version == "5.0.0") {
						if !strings.Contains(r.URL.Path, "/doc/user/_update") {
							t.Errorf("unexpected typed update path %s", r.URL.Path)
						}
					} else if !strings.Contains(r.URL.Path, "/_update/user") {
						t.Errorf("unexpected typeless update path %s", r.URL.Path)
					}
					if r.URL.Query().Get("refresh") != orm.WaitForRefresh {
						t.Errorf("refresh was not preserved: %s", r.URL.RawQuery)
					}
					var body struct {
						Upsert bool `json:"doc_as_upsert"`
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						t.Error(err)
						w.WriteHeader(500)
						return
					}
					wantUpsert := mode == "upsert" || mode == "direct"
					if body.Upsert != wantUpsert {
						t.Errorf("doc_as_upsert = %v, want %v", body.Upsert, wantUpsert)
					}
					if !exists && !body.Upsert {
						w.WriteHeader(404)
						_, _ = w.Write([]byte(`{"error":{"type":"document_missing_exception"},"status":404}`))
						return
					}
					exists = true
					if mode == "noop" {
						_, _ = w.Write([]byte(`{"result":"noop","_shards":{"successful":0}}`))
					} else {
						_, _ = w.Write([]byte(`{"result":"updated"}`))
					}
				}))
				defer server.Close()
				cfg := elastic.ElasticsearchConfig{Endpoint: server.URL, Version: version, Distribution: target.distribution, Enabled: true}
				cfg.ID = t.Name()
				client, err := elastic.GetOrCreateClient(cfg)
				require.NoError(t, err)
				backend := &ElasticORM{Client: client}
				orm.Register(t.TempDir(), backend)
				account := &security.UserAccount{Name: "updated", Email: "alice@example.com"}
				account.ID = "user"
				ctx := orm.NewContext()
				ctx.Refresh = orm.WaitForRefresh
				switch mode {
				case "upsert":
					err = orm.Upsert(ctx, account)
				case "direct":
					_, err = client.Update(backend.GetIndexName(account), "", account.ID, account, ctx.Refresh)
				default:
					err = orm.Update(ctx, account)
				}
				if mode == "deleted" {
					require.Error(t, err)
				} else {
					require.NoError(t, err)
				}
				mutex.Lock()
				defer mutex.Unlock()
				require.Equal(t, 1, updates)
				require.Equal(t, mode != "deleted", exists)
			})
		}
	}
}
