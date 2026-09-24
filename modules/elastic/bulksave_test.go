/* Copyright © INFINI LTD. All rights reserved. */

package elastic

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"infini.sh/framework/core/elastic"
	api "infini.sh/framework/core/orm"
	"infini.sh/framework/modules/elastic/common"
)

type bulkSaveItem struct {
	ID   string `json:"id,omitempty" elastic_meta:"_id" elastic_mapping:"id: { type: keyword }"`
	Name string `json:"name,omitempty"`
}

// esStub is a minimal offline Elasticsearch: it records requests and answers
// index/bulk/refresh the way the adapter expects.
type esStub struct {
	*httptest.Server
	mu       sync.Mutex
	requests [][2]string //method, path
	lastBulk []byte
	failBulk bool
}

func newESStub() *esStub {
	s := &esStub{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		isBulk := strings.Contains(r.URL.Path, "/_bulk")
		s.mu.Lock()
		s.requests = append(s.requests, [2]string{r.Method, r.URL.Path})
		if isBulk {
			s.lastBulk = body
		}
		fail := s.failBulk
		s.mu.Unlock()

		w.Header().Set("Content-Type", "application/json")
		switch {
		case isBulk && fail:
			_, _ = w.Write([]byte(`{"errors":true,"items":[{"index":{"status":429,"error":{"type":"es_rejected_execution_exception","reason":"stub injected"}}}]}`))
		case isBulk:
			_, _ = w.Write([]byte(`{}`))
		case strings.HasSuffix(r.URL.Path, "/_refresh"):
			_, _ = w.Write([]byte(`{"_shards":{}}`))
		default:
			_, _ = w.Write([]byte(`{"_id":"stub","result":"created"}`))
		}
	}))
	return s
}

func (s *esStub) setFailBulk(v bool) {
	s.mu.Lock()
	s.failBulk = v
	s.mu.Unlock()
}

func (s *esStub) allRequests() [][2]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([][2]string, len(s.requests))
	copy(out, s.requests)
	return out
}

func (s *esStub) bulkBody() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastBulk
}

// newStubClient builds a self-contained elastic API client against an
// arbitrary endpoint: a preset version selects the adapter without probing,
// so the whole thing works offline.
func newStubClient(t testing.TB, endpoint string) elastic.API {
	elastic.ResetClientCacheForTest()
	elastic.RegisterClientProvider(common.InitClientWithConfig)

	cfg := elastic.ElasticsearchConfig{}
	cfg.ID = "bulksave-stub"
	cfg.Name = "bulksave-stub"
	cfg.Endpoint = endpoint
	cfg.Distribution = elastic.Elasticsearch
	cfg.Version = "8.0.0"
	cfg.Enabled = true

	c, err := elastic.GetOrCreateClient(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func bulkBatch(n int, prefix string) []interface{} {
	out := make([]interface{}, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, &bulkSaveItem{ID: fmt.Sprintf("%s-%03d", prefix, i), Name: "bench"})
	}
	return out
}

func TestElasticORMBulkSaveSingleRequest(t *testing.T) {
	stub := newESStub()
	defer stub.Close()
	handler := &ElasticORM{Client: newStubClient(t, stub.URL)}

	const n = 50
	if err := handler.BulkSave(api.NewContext(), bulkBatch(n, "bulk")); err != nil {
		t.Fatal(err)
	}

	requests := stub.allRequests()
	if len(requests) != 1 {
		t.Fatalf("a batch of %d docs must be exactly ONE request, got %v", n, requests)
	}
	if method, path := requests[0][0], requests[0][1]; method != http.MethodPost || !strings.Contains(path, "/_bulk") {
		t.Fatalf("expected a single POST /_bulk, got %v %v", method, path)
	}

	//NDJSON: n action/doc pairs; every action carries the doc's _index and _id
	lines := strings.Split(strings.TrimSpace(string(stub.bulkBody())), "\n")
	if len(lines) != 2*n {
		t.Fatalf("expected %d ndjson lines, got %d", 2*n, len(lines))
	}
	for i := 0; i < n; i++ {
		var action struct {
			Index struct {
				Index string `json:"_index"`
				ID    string `json:"_id"`
			} `json:"index"`
		}
		if err := json.Unmarshal([]byte(lines[2*i]), &action); err != nil {
			t.Fatalf("action line %d: %v", i, err)
		}
		if action.Index.Index == "" || action.Index.ID != fmt.Sprintf("bulk-%03d", i) {
			t.Fatalf("action line %d malformed: %+v", i, action.Index)
		}
		var doc bulkSaveItem
		if err := json.Unmarshal([]byte(lines[2*i+1]), &doc); err != nil {
			t.Fatalf("doc line %d: %v", i, err)
		}
		if doc.ID != action.Index.ID {
			t.Fatalf("doc %d id %q does not match action _id %q", i, doc.ID, action.Index.ID)
		}
	}
}

// The per-row path the batch replaces: N docs through Save are N requests.
func TestElasticORMSavePerRowRequestCount(t *testing.T) {
	stub := newESStub()
	defer stub.Close()
	handler := &ElasticORM{Client: newStubClient(t, stub.URL)}

	for _, o := range bulkBatch(20, "row") {
		if err := handler.Save(nil, o); err != nil {
			t.Fatal(err)
		}
	}
	if got := len(stub.allRequests()); got != 20 {
		t.Fatalf("20 per-row Saves must be 20 requests, got %v", got)
	}
}

func TestElasticORMBulkSaveRefreshesOncePerIndex(t *testing.T) {
	stub := newESStub()
	defer stub.Close()
	handler := &ElasticORM{Client: newStubClient(t, stub.URL)}

	ctx := api.NewContext()
	ctx.Refresh = "true"
	if err := handler.BulkSave(ctx, bulkBatch(10, "refresh")); err != nil {
		t.Fatal(err)
	}

	bulk, refresh := 0, 0
	for _, r := range stub.allRequests() {
		if strings.Contains(r[1], "/_bulk") {
			bulk++
		}
		if strings.HasSuffix(r[1], "/_refresh") {
			refresh++
		}
	}
	if bulk != 1 || refresh != 1 {
		t.Fatalf("expected 1 bulk + 1 refresh for a single index, got bulk=%d refresh=%d", bulk, refresh)
	}
}

func TestElasticORMBulkSavePartialFailure(t *testing.T) {
	stub := newESStub()
	defer stub.Close()
	stub.setFailBulk(true)
	handler := &ElasticORM{Client: newStubClient(t, stub.URL)}

	if err := handler.BulkSave(api.NewContext(), bulkBatch(5, "fail")); err == nil {
		t.Fatal("partial bulk failure must surface as an error")
	}
}

func TestElasticORMBulkSaveRequiresID(t *testing.T) {
	stub := newESStub()
	defer stub.Close()
	handler := &ElasticORM{Client: newStubClient(t, stub.URL)}

	batch := []interface{}{&bulkSaveItem{ID: "ok"}, &bulkSaveItem{}} //second row: no id
	if err := handler.BulkSave(api.NewContext(), batch); err == nil {
		t.Fatal("missing id must fail the batch")
	}
	if got := len(stub.allRequests()); got != 0 {
		t.Fatalf("no request may be sent when a row lacks an id, got %d", got)
	}
}

const benchBatchSize = 400

func BenchmarkElasticORM_Save_PerRow(b *testing.B) {
	stub := newESStub()
	defer stub.Close()
	handler := &ElasticORM{Client: newStubClient(b, stub.URL)}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, o := range bulkBatch(benchBatchSize, "row") {
			if err := handler.Save(nil, o); err != nil {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkElasticORM_BulkSave(b *testing.B) {
	stub := newESStub()
	defer stub.Close()
	handler := &ElasticORM{Client: newStubClient(b, stub.URL)}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := handler.BulkSave(api.NewContext(), bulkBatch(benchBatchSize, "bulk")); err != nil {
			b.Fatal(err)
		}
	}
}
