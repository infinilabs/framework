/* Copyright © INFINI LTD. All rights reserved. */

package sqlite

import (
	"fmt"
	"path/filepath"
	"testing"

	"infini.sh/framework/core/orm"
)

// Both benchmarks write the same batch of rows to a real on-disk sqlite
// database: per-row Save opens and commits one transaction per row (one WAL
// commit/fsync each), BulkSave commits the whole batch once.

const benchBatchSize = 400

func benchHandler(b *testing.B) *SQLiteORM {
	handler := &SQLiteORM{Config: SQLiteConfig{
		Enabled: true,
		DBPath:  filepath.Join(b.TempDir(), "bench.db"),
	}}
	if err := handler.Open(); err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { handler.Close() })
	if err := handler.RegisterSchemaWithName(TestItem{}, "bench_items"); err != nil {
		b.Fatal(err)
	}
	return handler
}

func benchBatch(n int) []interface{} {
	out := make([]interface{}, 0, n)
	for i := 0; i < n; i++ {
		item := &TestItem{}
		item.ID = fmt.Sprintf("bench-%04d", i)
		out = append(out, item)
	}
	return out
}

func BenchmarkSQLiteORM_Save_PerRow(b *testing.B) {
	handler := benchHandler(b)
	ctx := orm.NewContext()
	orm.WithModel(ctx, &TestItem{})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, o := range benchBatch(benchBatchSize) {
			if err := handler.Save(ctx, o); err != nil {
				b.Fatal(err)
			}
		}
	}
}

func BenchmarkSQLiteORM_BulkSave(b *testing.B) {
	handler := benchHandler(b)
	ctx := orm.NewContext()
	orm.WithModel(ctx, &TestItem{})
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := handler.BulkSave(ctx, benchBatch(benchBatchSize)); err != nil {
			b.Fatal(err)
		}
	}
}
