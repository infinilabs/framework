/* Copyright © INFINI LTD. All rights reserved. */

package easysearch

import (
	"testing"

	"infini.sh/framework/core/orm"
)

func TestParseSearchResponse(t *testing.T) {
	payload := `{"hits":{"total":{"value":2},"hits":[{"_id":"a","_source":{"id":"a","name":"A","distribution":"easysearch"}},{"_id":"b","_source":{"id":"b","name":"B"}}]}}`
	res, err := parseSearchResponse(&orm.SearchResult{Payload: []byte(payload)})
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if len(res.Hits.Hits) != 2 {
		t.Fatalf("expected 2 hits, got %d (%+v)", len(res.Hits.Hits), res.Hits.Hits)
	}
	var nameA, nameB string
	for _, hit := range res.Hits.Hits {
		if hit.ID == "a" {
			nameA, _ = hit.Source["name"].(string)
		}
		if hit.ID == "b" {
			nameB, _ = hit.Source["name"].(string)
		}
	}
	if nameA != "A" || nameB != "B" {
		t.Fatalf("hit sources mismatch: %q, %q", nameA, nameB)
	}
}

func TestParseSearchResponse_EmptyAndMalformed(t *testing.T) {
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
			res, err := parseSearchResponse(c.res)
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

func TestTruncate(t *testing.T) {
	cases := []struct {
		in   string
		n    int
		want string
	}{
		{"abc", 5, "abc"},     // under limit → unchanged
		{"abcdef", 3, "abc…"}, // over limit → cut + ellipsis
		{"世界你好", 2, "世界…"},    // rune-aware, not byte-aware
		{"", 3, ""},           // empty
	}
	for _, c := range cases {
		if got := truncate(c.in, c.n); got != c.want {
			t.Errorf("truncate(%q,%d) = %q, want %q", c.in, c.n, got, c.want)
		}
	}
}
