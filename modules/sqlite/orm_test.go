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
	"infini.sh/framework/core/util"
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

// Tests below verify that the SQLite backend correctly handles the patterns
// used by RegisterDataOperationPreHook for Create, Update, and Save operations,
// including system field round-trip and ownership queries.

func TestSQLiteORM_SystemField_CreateAndGet(t *testing.T) {
	handler, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	item := &TestItem{}
	item.ID = "sys-1"
	item.Name = "OwnedDoc"
	item.Status = "active"
	item.Age = 25
	item.Created = &now
	item.Updated = &now

	// Simulate what the Create pre-hook does: set _system.owner_id
	item.SetSystemValue(orm.OwnerIDKey, "user-abc")

	err := handler.Create(nil, item)
	require.NoError(t, err)

	// Get it back and verify system fields survived the round-trip
	fetched := &TestItem{}
	fetched.ID = "sys-1"
	exists, err := handler.Get(nil, fetched)
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "OwnedDoc", fetched.Name)
	assert.Equal(t, "user-abc", fetched.GetOwnerID())
	assert.Equal(t, "user-abc", fetched.GetSystemString(orm.OwnerIDKey))
}

func TestSQLiteORM_SystemField_UpdatePreservesOwner(t *testing.T) {
	handler, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	item := &TestItem{}
	item.ID = "sys-2"
	item.Name = "Doc"
	item.Status = "active"
	item.Age = 30
	item.Created = &now
	item.Updated = &now
	item.SetSystemValue(orm.OwnerIDKey, "user-xyz")

	err := handler.Create(nil, item)
	require.NoError(t, err)

	// Fetch the object (simulates how Update hook would read it)
	fetched := &TestItem{}
	fetched.ID = "sys-2"
	exists, err := handler.Get(nil, fetched)
	require.NoError(t, err)
	assert.True(t, exists)

	// Verify GetOwnerID works after deserialization (used by Update/Save hook)
	ownerID := orm.GetOwnerID(fetched)
	assert.Equal(t, "user-xyz", ownerID)

	// Simulate update: modify name but keep system fields
	fetched.Name = "Updated Doc"
	err = handler.Update(nil, fetched)
	require.NoError(t, err)

	// Verify system fields are preserved after update
	fetched2 := &TestItem{}
	fetched2.ID = "sys-2"
	exists, err = handler.Get(nil, fetched2)
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "Updated Doc", fetched2.Name)
	assert.Equal(t, "user-xyz", fetched2.GetOwnerID())
}

func TestSQLiteORM_SystemField_SaveUpsertWithOwner(t *testing.T) {
	handler, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	item := &TestItem{}
	item.ID = "sys-3"
	item.Name = "SaveDoc"
	item.Status = "pending"
	item.Age = 40
	item.Created = &now
	item.Updated = &now

	// Simulate what the Update/Save hook does when owner_id is empty:
	// assign current user as owner
	var accessor orm.SystemFieldAccessor = item
	accessor.SetSystemValue(orm.OwnerIDKey, "user-save-1")

	err := handler.Save(nil, item)
	require.NoError(t, err)

	// Verify round-trip
	fetched := &TestItem{}
	fetched.ID = "sys-3"
	exists, err := handler.Get(nil, fetched)
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "SaveDoc", fetched.Name)
	assert.Equal(t, "user-save-1", fetched.GetOwnerID())

	// Save again with updated data (upsert)
	fetched.Name = "SaveDoc Updated"
	err = handler.Save(nil, fetched)
	require.NoError(t, err)

	fetched2 := &TestItem{}
	fetched2.ID = "sys-3"
	exists, err = handler.Get(nil, fetched2)
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "SaveDoc Updated", fetched2.Name)
	assert.Equal(t, "user-save-1", fetched2.GetOwnerID())
}

func TestSQLiteORM_SystemField_GetOwnerIDFromInterface(t *testing.T) {
	handler, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	item := &TestItem{}
	item.ID = "sys-4"
	item.Name = "InterfaceTest"
	item.Status = "active"
	item.Age = 50
	item.Created = &now
	item.Updated = &now
	item.SetSystemValue(orm.OwnerIDKey, "user-interface")

	err := handler.Create(nil, item)
	require.NoError(t, err)

	fetched := &TestItem{}
	fetched.ID = "sys-4"
	exists, err := handler.Get(nil, fetched)
	require.NoError(t, err)
	assert.True(t, exists)

	// Test GetOwnerID through the global function (as used by the Update/Save hook)
	ownerID := orm.GetOwnerID(fetched)
	assert.Equal(t, "user-interface", ownerID)

	// Test Object interface (GetID)
	obj, ok := interface{}(fetched).(orm.Object)
	require.True(t, ok, "TestItem must implement orm.Object")
	assert.Equal(t, "sys-4", obj.GetID())

	// Test SystemFieldAccessor interface
	accessor, ok := interface{}(fetched).(orm.SystemFieldAccessor)
	require.True(t, ok, "TestItem must implement orm.SystemFieldAccessor")
	assert.Equal(t, "user-interface", accessor.GetOwnerID())
}

func TestSQLiteORM_SearchV2_FilterByOwnerID(t *testing.T) {
	handler, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	items := []*TestItem{
		{ORMObjectBase: orm.ORMObjectBase{ID: "own-1", Created: &now, Updated: &now}, Name: "Alice Doc", Status: "active", Age: 30},
		{ORMObjectBase: orm.ORMObjectBase{ID: "own-2", Created: &now, Updated: &now}, Name: "Bob Doc", Status: "active", Age: 25},
		{ORMObjectBase: orm.ORMObjectBase{ID: "own-3", Created: &now, Updated: &now}, Name: "Charlie Doc", Status: "active", Age: 35},
	}
	items[0].SetSystemValue(orm.OwnerIDKey, "user-alice")
	items[1].SetSystemValue(orm.OwnerIDKey, "user-bob")
	items[2].SetSystemValue(orm.OwnerIDKey, "user-alice")

	for _, item := range items {
		err := handler.Create(nil, item)
		require.NoError(t, err)
	}

	// SearchV2 filtering by _system.owner_id (mirrors search hook pattern)
	ctx := orm.NewContext()
	orm.WithModel(ctx, &TestItem{})

	qb := orm.NewQuery()
	qb.Filter(orm.TermQuery("_system.owner_id", "user-alice"))
	qb.Size(10)

	searchResult, err := handler.SearchV2(ctx, qb)
	require.NoError(t, err)
	assert.NotNil(t, searchResult)
	assert.Equal(t, 200, searchResult.Status)

	// Parse response to verify count
	var response map[string]interface{}
	err = util.FromJSONBytes(searchResult.Payload.([]byte), &response)
	require.NoError(t, err)
	hits := response["hits"].(map[string]interface{})
	total := hits["total"].(map[string]interface{})
	assert.Equal(t, float64(2), total["value"])
}

func TestSQLiteORM_SearchV2_ShouldOwnerOrSharedIDs(t *testing.T) {
	handler, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	items := []*TestItem{
		{ORMObjectBase: orm.ORMObjectBase{ID: "sh-1", Created: &now, Updated: &now}, Name: "Doc1", Status: "active"},
		{ORMObjectBase: orm.ORMObjectBase{ID: "sh-2", Created: &now, Updated: &now}, Name: "Doc2", Status: "active"},
		{ORMObjectBase: orm.ORMObjectBase{ID: "sh-3", Created: &now, Updated: &now}, Name: "Doc3", Status: "active"},
		{ORMObjectBase: orm.ORMObjectBase{ID: "sh-4", Created: &now, Updated: &now}, Name: "Doc4", Status: "active"},
	}
	items[0].SetSystemValue(orm.OwnerIDKey, "user-A")
	items[1].SetSystemValue(orm.OwnerIDKey, "user-B")
	items[2].SetSystemValue(orm.OwnerIDKey, "user-A")
	items[3].SetSystemValue(orm.OwnerIDKey, "user-C")

	for _, item := range items {
		err := handler.Create(nil, item)
		require.NoError(t, err)
	}

	// Simulate the search hook pattern: owner_id OR shared IDs
	ctx := orm.NewContext()
	orm.WithModel(ctx, &TestItem{})

	qb := orm.NewQuery()
	bq := orm.ShouldQuery()
	bq.ShouldClauses = append(bq.ShouldClauses, orm.TermQuery("_system.owner_id", "user-A"))
	bq.ShouldClauses = append(bq.ShouldClauses, orm.TermsQuery("id", []string{"sh-2"}))
	bq.Parameter("minimum_should_match", 1)
	qb.Must(bq)
	qb.Size(10)

	searchResult, err := handler.SearchV2(ctx, qb)
	require.NoError(t, err)
	assert.NotNil(t, searchResult)

	// Parse response: should find sh-1 (owner), sh-3 (owner), sh-2 (shared)
	var response map[string]interface{}
	err = util.FromJSONBytes(searchResult.Payload.([]byte), &response)
	require.NoError(t, err)
	hits := response["hits"].(map[string]interface{})
	total := hits["total"].(map[string]interface{})
	assert.Equal(t, float64(3), total["value"])
}

func TestSQLiteORM_DeleteByQuery_ByOwnerID(t *testing.T) {
	handler, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	items := []*TestItem{
		{ORMObjectBase: orm.ORMObjectBase{ID: "del-1", Created: &now, Updated: &now}, Name: "Doc1"},
		{ORMObjectBase: orm.ORMObjectBase{ID: "del-2", Created: &now, Updated: &now}, Name: "Doc2"},
		{ORMObjectBase: orm.ORMObjectBase{ID: "del-3", Created: &now, Updated: &now}, Name: "Doc3"},
	}
	items[0].SetSystemValue(orm.OwnerIDKey, "user-del")
	items[1].SetSystemValue(orm.OwnerIDKey, "user-keep")
	items[2].SetSystemValue(orm.OwnerIDKey, "user-del")

	for _, item := range items {
		err := handler.Create(nil, item)
		require.NoError(t, err)
	}

	ctx := orm.NewContext()
	orm.WithModel(ctx, &TestItem{})

	qb := orm.NewQuery()
	qb.Filter(orm.TermQuery("_system.owner_id", "user-del"))

	resp, err := handler.DeleteByQuery(ctx, qb)
	require.NoError(t, err)
	assert.Equal(t, int64(2), resp.Deleted)

	// Verify only user-keep's doc remains
	count, err := handler.Count(&TestItem{}, nil)
	require.NoError(t, err)
	assert.Equal(t, int64(1), count)

	fetched := &TestItem{}
	fetched.ID = "del-2"
	exists, err := handler.Get(nil, fetched)
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "user-keep", fetched.GetOwnerID())
}

func TestSQLiteORM_SearchV2_SingleShouldOwnerIsMandatory(t *testing.T) {
	handler, cleanup := setupTestDB(t)
	defer cleanup()

	now := time.Now()
	items := []*TestItem{
		{ORMObjectBase: orm.ORMObjectBase{ID: "msm-1", Created: &now, Updated: &now}, Name: "Doc1", Status: "active"},
		{ORMObjectBase: orm.ORMObjectBase{ID: "msm-2", Created: &now, Updated: &now}, Name: "Doc2", Status: "active"},
		{ORMObjectBase: orm.ORMObjectBase{ID: "msm-3", Created: &now, Updated: &now}, Name: "Doc3", Status: "active"},
	}
	items[0].SetSystemValue(orm.OwnerIDKey, "user-owner")
	items[1].SetSystemValue(orm.OwnerIDKey, "user-other")
	items[2].SetSystemValue(orm.OwnerIDKey, "user-owner")

	for _, item := range items {
		err := handler.Create(nil, item)
		require.NoError(t, err)
	}

	// Simulate the search hook pattern when sharing is disabled:
	// single should clause with minimum_should_match=1 means owner_id MUST match
	ctx := orm.NewContext()
	orm.WithModel(ctx, &TestItem{})

	qb := orm.NewQuery()
	bq := orm.ShouldQuery()
	bq.ShouldClauses = append(bq.ShouldClauses, orm.TermQuery("_system.owner_id", "user-owner"))
	bq.Parameter("minimum_should_match", 1)
	qb.Must(bq)
	qb.Size(10)

	searchResult, err := handler.SearchV2(ctx, qb)
	require.NoError(t, err)
	assert.NotNil(t, searchResult)

	// Parse response: only owner's docs should be returned (msm-1, msm-3)
	var response map[string]interface{}
	err = util.FromJSONBytes(searchResult.Payload.([]byte), &response)
	require.NoError(t, err)
	hits := response["hits"].(map[string]interface{})
	total := hits["total"].(map[string]interface{})
	assert.Equal(t, float64(2), total["value"],
		"single should clause with minimum_should_match=1 must act as mandatory filter")
}
