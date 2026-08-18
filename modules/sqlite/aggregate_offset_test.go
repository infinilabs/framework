/* Copyright © INFINI LTD. All rights reserved. */

package sqlite

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"infini.sh/framework/core/orm"
)

type offsetDoc struct {
	orm.ORMObjectBase
	TS    time.Time `json:"ts,omitempty" elastic_mapping:"ts: { type: date }"`
	Count int64     `json:"count,omitempty" elastic_mapping:"count: { type: long }"`
}

// TestAggregate_DateHistogramOffset proves offset bucketing reproduces
// logpilot's now-anchored age slotting: slot(t) = floor((now-t)/1h) equals
// the histogram bucket of t with offset = now mod 1h, mapped via each
// bucket's right edge. Fixture instants avoid the exact boundary sliver
// (seconds truncation of the minute-granularity offset).
func TestAggregate_DateHistogramOffset(t *testing.T) {
	handler := &SQLiteORM{Config: SQLiteConfig{Enabled: true, DBPath: filepath.Join(t.TempDir(), "off.db")}}
	require.NoError(t, handler.Open())
	require.NoError(t, handler.RegisterSchemaWithName(offsetDoc{}, "offset_docs"))
	defer handler.Close()

	now := time.Date(2026, 8, 13, 10, 37, 12, 0, time.UTC)
	offset := now.Sub(now.Truncate(time.Hour))

	seed := []struct {
		minutesBack int
		count       int64
		wantSlot    int // age slot, 0 = newest
	}{
		{5, 3, 0},    // 10:32
		{61, 5, 1},   // 09:36
		{90, 7, 1},   // 09:07
		{182, 11, 3}, // 07:35
	}
	for i, s := range seed {
		d := offsetDoc{TS: now.Add(-time.Duration(s.minutesBack) * time.Minute), Count: s.count}
		d.ID = fmt.Sprintf("d%d", i)
		require.NoError(t, handler.Save(nil, &d))
	}

	dh := &orm.DateHistogramAggregation{Field: "ts", Interval: "1h", Offset: offset}
	dh.AddNested("total", &orm.MetricAggregation{Type: orm.MetricSum, Field: "count"})
	ctx := orm.NewContext()
	orm.WithModel(ctx, &offsetDoc{})
	qb := orm.NewQuery()
	qb.SetAggregations(map[string]orm.Aggregation{"h": dh})
	res, err := handler.Aggregate(ctx, qb)
	require.NoError(t, err)

	got := map[int]int64{}
	for _, b := range res.Aggs["h"].Buckets {
		require.NotNil(t, b.Aggs["total"], "zero-filled buckets must carry metric nodes")
		// Map via the bucket's right edge: slot = floor((now - (start+1h))/h).
		rightEdge := time.UnixMilli(b.KeyRaw.(int64)).Add(time.Hour)
		slot := int(now.Sub(rightEdge).Hours())
		got[slot] += int64(b.Aggs["total"].Value)
	}
	want := map[int]int64{2: 0} // slot 2 zero-filled (gap hour, min_doc_count:0)
	for _, s := range seed {
		want[s.wantSlot] += s.count
	}
	assert.Equal(t, want, got)
}
