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

package sqlite

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"infini.sh/framework/core/orm"
)

// TestItem is a sample struct for testing ORM operations.
type TestItem struct {
	orm.ORMObjectBase
	Name   string `json:"name,omitempty" elastic_mapping:"name: { type: keyword }"`
	Status string `json:"status,omitempty" elastic_mapping:"status: { type: keyword }"`
	Age    int    `json:"age,omitempty" elastic_mapping:"age: { type: integer }"`
}

func setupTestDB(t *testing.T) (*SQLiteORM, func()) {
	t.Helper()
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	handler := &SQLiteORM{
		Config: SQLiteConfig{
			Enabled: true,
			DBPath:  dbPath,
		},
	}

	err := handler.Open()
	require.NoError(t, err)

	err = handler.RegisterSchemaWithName(TestItem{}, "test_items")
	require.NoError(t, err)

	cleanup := func() {
		handler.Close()
		os.RemoveAll(tmpDir)
	}
	return handler, cleanup
}

func TestSQLiteORM_CRUD(t *testing.T) {
	handler, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	item := &TestItem{}
	item.ID = "test-1"
	item.Name = "Alice"
	item.Status = "active"
	item.Age = 30
	item.Created = &now
	item.Updated = &now

	// Create
	err := handler.Create(nil, item)
	assert.NoError(t, err)

	// Get
	fetched := &TestItem{}
	fetched.ID = "test-1"
	exists, err := handler.Get(nil, fetched)
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "Alice", fetched.Name)
	assert.Equal(t, "active", fetched.Status)
	assert.Equal(t, 30, fetched.Age)

	// Update
	item.Name = "Alice Updated"
	err = handler.Update(nil, item)
	assert.NoError(t, err)

	fetched2 := &TestItem{}
	fetched2.ID = "test-1"
	exists, err = handler.Get(nil, fetched2)
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "Alice Updated", fetched2.Name)

	// Save (upsert)
	item2 := &TestItem{}
	item2.ID = "test-2"
	item2.Name = "Bob"
	item2.Status = "pending"
	item2.Age = 25
	item2.Created = &now
	item2.Updated = &now

	err = handler.Save(nil, item2)
	assert.NoError(t, err)

	fetched3 := &TestItem{}
	fetched3.ID = "test-2"
	exists, err = handler.Get(nil, fetched3)
	assert.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "Bob", fetched3.Name)

	// Count
	count, err := handler.Count(&TestItem{}, nil)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)

	// Delete
	deleteItem := &TestItem{}
	deleteItem.ID = "test-1"
	err = handler.Delete(nil, deleteItem)
	assert.NoError(t, err)

	count, err = handler.Count(&TestItem{}, nil)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), count)

	// Get non-existent
	missing := &TestItem{}
	missing.ID = "test-1"
	exists, err = handler.Get(nil, missing)
	assert.Error(t, err)
	assert.False(t, exists)
}

func TestSQLiteORM_Search(t *testing.T) {
	handler, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	items := []*TestItem{
		{ORMObjectBase: orm.ORMObjectBase{ID: "s1", Created: &now, Updated: &now}, Name: "Alice", Status: "active", Age: 30},
		{ORMObjectBase: orm.ORMObjectBase{ID: "s2", Created: &now, Updated: &now}, Name: "Bob", Status: "pending", Age: 25},
		{ORMObjectBase: orm.ORMObjectBase{ID: "s3", Created: &now, Updated: &now}, Name: "Charlie", Status: "active", Age: 35},
	}

	for _, item := range items {
		err := handler.Create(nil, item)
		require.NoError(t, err)
	}

	// Search with condition
	q := &orm.Query{
		Conds: orm.And(orm.Eq("status", "active")),
		Size:  10,
	}
	err, result := handler.Search(&TestItem{}, q)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), result.Total)

	// GetBy
	err, result = handler.GetBy("name", "Bob", &TestItem{})
	assert.NoError(t, err)
	assert.Equal(t, int64(1), result.Total)
}

func TestSQLiteORM_SearchV2(t *testing.T) {
	handler, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	items := []*TestItem{
		{ORMObjectBase: orm.ORMObjectBase{ID: "v1", Created: &now, Updated: &now}, Name: "Alice", Status: "active", Age: 30},
		{ORMObjectBase: orm.ORMObjectBase{ID: "v2", Created: &now, Updated: &now}, Name: "Bob", Status: "pending", Age: 25},
		{ORMObjectBase: orm.ORMObjectBase{ID: "v3", Created: &now, Updated: &now}, Name: "Charlie", Status: "active", Age: 35},
	}

	for _, item := range items {
		err := handler.Create(nil, item)
		require.NoError(t, err)
	}

	ctx := orm.NewContext()
	orm.WithModel(ctx, &TestItem{})

	qb := orm.NewQuery()
	qb.Filter(orm.TermQuery("status", "active"))
	qb.Size(10)

	searchResult, err := handler.SearchV2(ctx, qb)
	assert.NoError(t, err)
	assert.NotNil(t, searchResult)
	assert.Equal(t, 200, searchResult.Status)
}

func TestSQLiteORM_DeleteByQuery(t *testing.T) {
	handler, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	items := []*TestItem{
		{ORMObjectBase: orm.ORMObjectBase{ID: "d1", Created: &now, Updated: &now}, Name: "Alice", Status: "active", Age: 30},
		{ORMObjectBase: orm.ORMObjectBase{ID: "d2", Created: &now, Updated: &now}, Name: "Bob", Status: "pending", Age: 25},
		{ORMObjectBase: orm.ORMObjectBase{ID: "d3", Created: &now, Updated: &now}, Name: "Charlie", Status: "active", Age: 35},
	}

	for _, item := range items {
		err := handler.Create(nil, item)
		require.NoError(t, err)
	}

	ctx := orm.NewContext()
	orm.WithModel(ctx, &TestItem{})

	qb := orm.NewQuery()
	qb.Filter(orm.TermQuery("status", "pending"))

	resp, err := handler.DeleteByQuery(ctx, qb)
	assert.NoError(t, err)
	assert.NotNil(t, resp)
	assert.Equal(t, int64(1), resp.Deleted)

	// Verify remaining count
	count, err := handler.Count(&TestItem{}, nil)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), count)
}

func TestSQLiteORM_GetIndexName(t *testing.T) {
	handler := &SQLiteORM{}
	initTableName(TestItem{}, "test_items")
	name := handler.GetIndexName(&TestItem{})
	assert.Equal(t, "test_items", name)
}
