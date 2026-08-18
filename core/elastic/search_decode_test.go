/* Copyright © INFINI LTD. All rights reserved. */

package elastic

import (
	"testing"

	"infini.sh/framework/core/orm"
)

type decodeTestItem struct {
	orm.ORMObjectBase
	Name string `json:"name,omitempty"`
}

type decodePlainItem struct { // no orm.Object — relies on "id" injection
	ID   string `json:"id,omitempty"`
	Name string `json:"name,omitempty"`
}

func TestDecodeSearchResult(t *testing.T) {
	payload := `{"hits":{"total":{"value":2},"hits":[{"_id":"a","_source":{"id":"a","name":"A"}},{"_id":"b","_source":{"id":"b","name":"B"}}]}}`
	res, err := DecodeSearchResult(&orm.SearchResult{Payload: []byte(payload)})
	if err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if got := res.GetTotal(); got != 2 {
		t.Fatalf("total = %d, want 2", got)
	}
	if len(res.Hits.Hits) != 2 {
		t.Fatalf("hits = %d, want 2", len(res.Hits.Hits))
	}
	if res.Hits.Hits[0].Source["name"] != "A" || res.Hits.Hits[1].ID != "b" {
		t.Fatalf("hit contents mismatch: %+v", res.Hits.Hits)
	}
}

func TestDecodeSearchResult_EmptyAndMalformed(t *testing.T) {
	checks := []struct {
		name    string
		res     *orm.SearchResult
		wantErr bool
	}{
		{"nil result", nil, false},
		{"nil payload", &orm.SearchResult{}, false},
		{"empty hits", &orm.SearchResult{Payload: []byte(`{"hits":{"hits":[]}}`)}, false},
		{"empty bytes", &orm.SearchResult{Payload: []byte(``)}, false},
		{"string payload", &orm.SearchResult{Payload: `{"hits":{"hits":[]}}`}, false},
		{"empty string", &orm.SearchResult{Payload: ``}, false},
		{"non-bytes payload", &orm.SearchResult{Payload: 12345}, false},
		{"malformed json", &orm.SearchResult{Payload: []byte(`not json`)}, true},
	}
	for _, c := range checks {
		t.Run(c.name, func(t *testing.T) {
			res, err := DecodeSearchResult(c.res)
			if c.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got %+v", res)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(res.Hits.Hits) != 0 {
				t.Fatalf("expected no hits, got %d", len(res.Hits.Hits))
			}
		})
	}
}

func TestDecodeHits(t *testing.T) {
	payload := `{"hits":{"total":{"value":2},"hits":[` +
		`{"_id":"a","_source":{"name":"A"}},` +
		`{"_id":"b","_source":{"name":"B"}}]}}`

	t.Run("typed with orm.Object", func(t *testing.T) {
		items, total, err := DecodeHits[decodeTestItem](&orm.SearchResult{Payload: []byte(payload)})
		if err != nil {
			t.Fatalf("DecodeHits: %v", err)
		}
		if total != 2 {
			t.Fatalf("total = %d, want 2", total)
		}
		if len(items) != 2 || items[0].Name != "A" || items[1].Name != "B" {
			t.Fatalf("items mismatch: %+v", items)
		}
		if items[0].ID != "a" || items[1].ID != "b" {
			t.Fatalf("_id not backfilled via SetID: %+v", items)
		}
	})

	t.Run("plain struct gets id injected", func(t *testing.T) {
		items, _, err := DecodeHits[decodePlainItem](&orm.SearchResult{Payload: []byte(payload)})
		if err != nil {
			t.Fatalf("DecodeHits: %v", err)
		}
		if len(items) != 2 || items[0].ID != "a" {
			t.Fatalf("id not injected into plain struct: %+v", items)
		}
	})

	t.Run("empty result yields non-nil slice", func(t *testing.T) {
		items, total, err := DecodeHits[decodeTestItem](&orm.SearchResult{Payload: []byte(`{"hits":{"total":{"value":0},"hits":[]}}`)})
		if err != nil {
			t.Fatalf("DecodeHits: %v", err)
		}
		if items == nil || len(items) != 0 {
			t.Fatalf("expected non-nil empty slice, got %#v", items)
		}
		if total != 0 {
			t.Fatalf("total = %d, want 0", total)
		}
	})

	t.Run("missing total reports unknown", func(t *testing.T) {
		// GetTotal() contract: -1 when the response carries no total.
		_, total, err := DecodeHits[decodeTestItem](&orm.SearchResult{Payload: []byte(`{"hits":{"hits":[]}}`)})
		if err != nil {
			t.Fatalf("DecodeHits: %v", err)
		}
		if total != -1 {
			t.Fatalf("total = %d, want -1 (unknown)", total)
		}
	})

	t.Run("nil source hit", func(t *testing.T) {
		payload := `{"hits":{"total":{"value":1},"hits":[{"_id":"x"}]}}`
		items, _, err := DecodeHits[decodeTestItem](&orm.SearchResult{Payload: []byte(payload)})
		if err != nil {
			t.Fatalf("DecodeHits: %v", err)
		}
		if len(items) != 1 || items[0].ID != "x" {
			t.Fatalf("nil _source hit mishandled: %+v", items)
		}
	})
}
