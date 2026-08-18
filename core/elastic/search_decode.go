/* Copyright © INFINI LTD. All rights reserved. */

package elastic

import (
	"infini.sh/framework/core/orm"
	"infini.sh/framework/core/util"
)

// ──────────────────────────────────────────────────────────────────────────
// Typed decoding of orm.SearchResult payloads.
//
// Every ORM backend returns search results as ES-shaped JSON
// ({"hits":{"total":...,"hits":[{"_id":...,"_source":{...}}]}}) carried in
// SearchResult.Payload. These helpers turn that convention into a typed
// contract so callers stop hand-rolling Payload assertions and per-entity
// parse helpers.
//
// Placement note: this file lives in core/elastic (not core/orm) because
// core/elastic already depends on core/orm for ORMObjectBase — the reverse
// import would be a cycle.
// ──────────────────────────────────────────────────────────────────────────

// DecodeSearchResult decodes an orm.SearchResult payload (ES-shaped JSON)
// into a SearchResponse. Payload accepts []byte, string, and nil/empty
// (which yield a zero response, not an error) so callers can pass results
// from any backend without defensive type checks.
func DecodeSearchResult(res *orm.SearchResult) (*SearchResponse, error) {
	out := &SearchResponse{}
	if res == nil {
		return out, nil
	}
	var raw []byte
	switch payload := res.Payload.(type) {
	case []byte:
		raw = payload
	case string:
		raw = []byte(payload)
	default:
		return out, nil
	}
	if len(raw) == 0 {
		return out, nil
	}
	if err := util.FromJSONBytes(raw, out); err != nil {
		return nil, err
	}
	return out, nil
}

// DecodeHits decodes the hits of an orm.SearchResult into a typed slice and
// returns it with the reported total. The slice is always non-nil (empty
// when there are no hits), so callers can range/append safely.
//
// Document IDs are backfilled from the ES _id field: when T implements
// orm.Object its SetID is called, and "id" is injected into the source map
// before decoding so plain structs with an `json:"id"` field work too.
func DecodeHits[T any](res *orm.SearchResult) ([]T, int64, error) {
	resp, err := DecodeSearchResult(res)
	if err != nil {
		return nil, 0, err
	}
	total := resp.GetTotal()

	hits := resp.Hits.Hits
	out := make([]T, 0, len(hits))
	for i := range hits {
		hit := &hits[i]

		src := hit.Source
		if src == nil {
			src = util.MapStr{}
		}
		// _id is the authoritative document ID; surface it on the decoded
		// value even when the stored source lacks an id field.
		if hit.ID != "" {
			src["id"] = hit.ID
		}

		raw, err := util.ToJSONBytes(src)
		if err != nil {
			return nil, total, err
		}
		var item T
		if err := util.FromJSONBytes(raw, &item); err != nil {
			return nil, total, err
		}
		if obj, ok := any(&item).(orm.Object); ok && hit.ID != "" {
			obj.SetID(hit.ID)
		}
		out = append(out, item)
	}
	return out, total, nil
}
