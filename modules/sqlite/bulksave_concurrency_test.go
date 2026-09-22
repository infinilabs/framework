/* Copyright © INFINI LTD. All rights reserved. */

package sqlite

// Concurrency regression: under the single-connection pool, a BulkSave's
// BEGIN/COMMIT bracket can interleave with other goroutines' plain Saves —
// those writes join the open transaction and are rolled back together if
// the batch fails. The test races a failing BulkSave against concurrent
// successful Saves and asserts no successful write is lost.

import (
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"infini.sh/framework/core/orm"
)

func TestBulkSave_ConcurrentWritesNotSwallowedByFailedBatch(t *testing.T) {
	handler := &SQLiteORM{Config: SQLiteConfig{
		Enabled: true,
		DBPath:  filepath.Join(t.TempDir(), "bulk-conc.db"),
	}}
	require.NoError(t, handler.Open())
	defer handler.Close()
	require.NoError(t, handler.RegisterSchemaWithName(TestItem{}, "bulk_conc"))
	ctx := orm.NewContext()
	orm.WithModel(ctx, &TestItem{})

	const concurrentWrites = 300
	const batchSize = 400

	stop := make(chan struct{})
	var wg sync.WaitGroup
	var mu sync.Mutex
	savedOK := 0

	// Writer goroutine: continuous single Saves, each verified successful.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < concurrentWrites; i++ {
			item := &TestItem{}
			item.ID = fmt.Sprintf("solo-%03d", i)
			if err := handler.Save(ctx, item); err != nil {
				t.Errorf("solo save %d: %v", i, err)
				return
			}
			mu.Lock()
			savedOK++
			mu.Unlock()
			select {
			case <-stop:
				return
			default:
			}
		}
	}()

	// Batch goroutine: a batch whose LAST item is invalid (empty id) forces
	// the whole transaction to roll back — any solo write that interleaved
	// into the open transaction would be swallowed with it.
	wg.Add(1)
	go func() {
		defer wg.Done()
		batch := make([]interface{}, 0, batchSize)
		for j := 0; j < batchSize-1; j++ {
			bj := &TestItem{}
			bj.ID = fmt.Sprintf("batch-%03d", j)
			batch = append(batch, bj)
		}
		bad := &TestItem{} // empty id: triggers rollback
		batch = append(batch, bad)
		if err := handler.BulkSave(ctx, batch); err == nil {
			t.Error("batch with empty-id item should fail")
		}
	}()

	wg.Wait()
	close(stop)

	mu.Lock()
	reported := savedOK
	mu.Unlock()

	// Every Save that returned success must be durably present.
	var present int
	require.NoError(t, handler.DB.QueryRow(
		`SELECT COUNT(*) FROM [bulk_conc] WHERE id LIKE 'solo-%'`).Scan(&present))
	assert.Equalf(t, reported, present,
		"%d solo writes reported success but only %d are present — writes were swallowed by the failed batch's rollback", reported, present)

	// The batch itself must be fully rolled back.
	var batchRows int
	require.NoError(t, handler.DB.QueryRow(
		`SELECT COUNT(*) FROM [bulk_conc] WHERE id LIKE 'batch-%'`).Scan(&batchRows))
	assert.Zero(t, batchRows, "failed batch must leave no rows")

	_ = time.Second // keep time import if assertions change
}
