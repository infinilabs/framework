/* Copyright © INFINI LTD. All rights reserved. */

// Package ormtest provides a shared contract-test suite that every ORM
// backend must satisfy, so sqlite and elastic keep producing equivalent
// (ES-shaped) results for the same QueryBuilder input. Backends wire it in
// with a one-line test:
//
//	func TestContract(t *testing.T) { ormtest.RunContractTests(t, newHandler) }
//
// The suite registers the handler globally (orm.Register) — run it once per
// test binary. Cases needing a live backend instance (elastic) carry the
// `integration` build tag at the call site, not here.
package ormtest

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"infini.sh/framework/core/elastic"
	"infini.sh/framework/core/orm"
)

// ContractModel is the canonical test document: scalars, a text field, a
// date, and system fields — enough to exercise filters, sorts, full-text
// and aggregations on every backend.
type ContractModel struct {
	orm.ORMObjectBase
	Name   string `json:"name,omitempty" elastic_mapping:"name: { type: keyword }"`
	Status string `json:"status,omitempty" elastic_mapping:"status: { type: keyword }"`
	Body   string `json:"body,omitempty" elastic_mapping:"body: { type: text }"`
	Age    int    `json:"age,omitempty" elastic_mapping:"age: { type: integer }"`
}

// RunContractTests executes the backend contract suite against a handler
// produced by factory. It registers the handler globally; callers must
// invoke it at most once per test binary.
func RunContractTests(t *testing.T, factory func() orm.ORM) {
	handler := factory()
	orm.Register("contract-test", handler)
	require.NoError(t, handler.RegisterSchemaWithName(ContractModel{}, "contract_docs"))
	seedContractData(t, handler)

	t.Run("CRUD roundtrip", func(t *testing.T) { contractCRUD(t, handler) })
	t.Run("filters", func(t *testing.T) { contractFilters(t, handler) })
	t.Run("sort and pagination", func(t *testing.T) { contractSortPaginate(t, handler) })
	t.Run("ES-shaped response", func(t *testing.T) { contractResponseShape(t, handler) })
	t.Run("partial update preserves fields", func(t *testing.T) { contractPartialUpdate(t, handler) })
	t.Run("terms aggregation", func(t *testing.T) { contractTermsAgg(t, handler) })
}

func seedContractData(t *testing.T, h orm.ORM) {
	t.Helper()
	now := time.Now().UTC()
	statuses := []string{"active", "pending", "archived"}
	for i := 0; i < 30; i++ {
		doc := ContractModel{
			Name:   fmt.Sprintf("doc-%02d", i),
			Status: statuses[i%3],
			Body:   fmt.Sprintf("body text number %d about sqlite and elastic", i),
			Age:    20 + i,
		}
		doc.ID = fmt.Sprintf("c%02d", i)
		doc.Created = &now
		require.NoError(t, h.Create(nil, &doc))
	}
}

func contractCRUD(t *testing.T, h orm.ORM) {
	doc := ContractModel{Name: "crud", Status: "active", Age: 1}
	doc.ID = "crud-1"
	require.NoError(t, h.Create(nil, &doc))

	got := ContractModel{}
	got.ID = "crud-1"
	exists, err := h.Get(nil, &got)
	require.NoError(t, err)
	assert.True(t, exists)
	assert.Equal(t, "crud", got.Name)

	got.Age = 42
	require.NoError(t, h.Update(nil, &got))
	got2 := ContractModel{}
	got2.ID = "crud-1"
	_, err = h.Get(nil, &got2)
	require.NoError(t, err)
	assert.Equal(t, 42, got2.Age)

	require.NoError(t, h.Delete(nil, &got2))
	got3 := ContractModel{}
	got3.ID = "crud-1"
	exists, _ = h.Get(nil, &got3)
	assert.False(t, exists)
}

func contractFilters(t *testing.T, h orm.ORM) {
	ctx := orm.NewContext()
	orm.WithModel(ctx, &ContractModel{})

	t.Run("term", func(t *testing.T) {
		res, err := h.SearchV2(ctx, orm.NewQuery().Filter(orm.TermQuery("status", "active")))
		require.NoError(t, err)
		items, _, err := elastic.DecodeHits[ContractModel](res)
		require.NoError(t, err)
		assert.Len(t, items, 10)
	})

	t.Run("range", func(t *testing.T) {
		res, err := h.SearchV2(ctx, orm.NewQuery().Filter(orm.Range("age").Gte(40)))
		require.NoError(t, err)
		items, total, err := elastic.DecodeHits[ContractModel](res)
		require.NoError(t, err)
		assert.Equal(t, int64(10), total)
		assert.Len(t, items, 10)
	})

	t.Run("term and range combined", func(t *testing.T) {
		res, err := h.SearchV2(ctx, orm.NewQuery().
			Filter(orm.TermQuery("status", "active")).
			Filter(orm.Range("age").Gte(20)).
			Filter(orm.Range("age").Lt(50)))
		require.NoError(t, err)
		items, _, err := elastic.DecodeHits[ContractModel](res)
		require.NoError(t, err)
		// active ages: 20,23,...,47 within [20,50) → all 10
		assert.Len(t, items, 10)
	})
}

func contractSortPaginate(t *testing.T, h orm.ORM) {
	ctx := orm.NewContext()
	orm.WithModel(ctx, &ContractModel{})

	qb := orm.NewQuery().
		SortBy(orm.Sort{Field: "age", SortType: orm.DESC}).
		From(0).Size(5)
	res, err := h.SearchV2(ctx, qb)
	require.NoError(t, err)
	items, _, err := elastic.DecodeHits[ContractModel](res)
	require.NoError(t, err)
	require.Len(t, items, 5)
	assert.Equal(t, 49, items[0].Age, "descending age sort")
	assert.Equal(t, 45, items[4].Age)

	// Page 2 continues the ordering.
	qb2 := orm.NewQuery().
		SortBy(orm.Sort{Field: "age", SortType: orm.DESC}).
		From(5).Size(5)
	res2, err := h.SearchV2(ctx, qb2)
	require.NoError(t, err)
	items2, _, err := elastic.DecodeHits[ContractModel](res2)
	require.NoError(t, err)
	require.Len(t, items2, 5)
	assert.Equal(t, 44, items2[0].Age)
}

func contractResponseShape(t *testing.T, h orm.ORM) {
	ctx := orm.NewContext()
	orm.WithModel(ctx, &ContractModel{})
	res, err := h.SearchV2(ctx, orm.NewQuery().Filter(orm.TermQuery("status", "pending")).Size(3))
	require.NoError(t, err)

	resp, err := elastic.DecodeSearchResult(res)
	require.NoError(t, err)
	assert.Equal(t, int64(10), resp.GetTotal())
	require.Len(t, resp.Hits.Hits, 3)
	for _, hit := range resp.Hits.Hits {
		assert.NotEmpty(t, hit.ID)
		assert.NotNil(t, hit.Source)
	}
}

func contractPartialUpdate(t *testing.T, h orm.ORM) {
	doc := ContractModel{Name: "before", Status: "active", Body: "keep me", Age: 7}
	doc.ID = "partial-1"
	require.NoError(t, h.Create(nil, &doc))

	obj := ContractModel{}
	obj.ID = "partial-1"
	require.NoError(t, orm.UpdatePartialFields(orm.NewContext(), &obj,
		map[string]interface{}{"name": "after"}))

	got := ContractModel{}
	got.ID = "partial-1"
	_, err := h.Get(nil, &got)
	require.NoError(t, err)
	assert.Equal(t, "after", got.Name, "delta field applied")
	assert.Equal(t, "active", got.Status, "untouched field preserved")
	assert.Equal(t, "keep me", got.Body, "untouched field preserved")
	assert.Equal(t, 7, got.Age, "untouched field preserved")
}

func contractTermsAgg(t *testing.T, h orm.ORM) {
	ctx := orm.NewContext()
	orm.WithModel(ctx, &ContractModel{})
	terms := &orm.TermsAggregation{Field: "status", Size: 10}
	qb := orm.NewQuery()
	qb.SetAggregations(map[string]orm.Aggregation{"by_status": terms})
	res, err := h.SearchV2(ctx, qb)
	require.NoError(t, err)

	resp, err := elastic.DecodeSearchResult(res)
	require.NoError(t, err)
	require.NotNil(t, resp.Aggregations)
	byStatus, ok := resp.Aggregations["by_status"]
	require.True(t, ok, "aggregations.by_status missing: %+v", resp.Aggregations)
	assert.Len(t, byStatus.Buckets, 3)
}
