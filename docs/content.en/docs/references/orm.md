---
title: "ORM (Object-Relational Mapping)"
weight: 50
---
# Object-Relational Mapping
The INFINI Framework provides a powerful ORM system with pluggable backends — Elasticsearch (including OpenSearch and Easysearch) and an embedded SQLite store — enabling developers to define, store, and query structured data objects with ease. The ORM handles object mapping, indexing, and provides a comprehensive set of CRUD operations, a backend-neutral query builder, and typed aggregations (metrics, buckets, and pipelines).

## Object Definition

Objects in the ORM are defined by embedding `orm.ORMObjectBase` in your struct, which automatically provides all required ORM functionality including system fields for metadata management. Objects must also implement the `Object` interface with `GetID()` and `SetID(string)` methods.

### Key Principle: ORMObjectBase Inheritance

**To make any struct ORM-capable, you must embed `orm.ORMObjectBase` as the first field.** This provides:
- System fields for metadata management
- Required interface implementations
- Automatic timestamp handling
- Built-in ID management

### Basic Object Structure

```go
// Basic ORM object with ORMObjectBase inheritance
type User struct {
    // EMBED ORMObjectBase as the first field - REQUIRED
    orm.ORMObjectBase        // Embedding ORM base for persistence-related fields

    // Custom fields
    Name      string    `json:"name"`
    Email     string    `json:"email"`
    Age       int       `json:"age"`
}

// Implement required Object interface methods (ORMObjectBase already handles GetID/SetID)
```

### Nested Inheritance: Building on Base Objects

The ORM supports nested inheritance, allowing you to create reusable base objects and extend them:

```go
// Define a base object with shared functionality
type CombinedFullText struct {
    orm.ORMObjectBase        // Embedding ORM base for persistence-related fields
    CombinedFullText  string `json:"-" elastic_mapping:"combined_fulltext:{type:text,index_prefixes:{},index_phrases:true,analyzer:combined_text_analyzer }"`

    Metadata map[string]interface{} `json:"metadata,omitempty" elastic_mapping:"metadata:{type:object}"` // Additional accessible metadata
    Payload  map[string]interface{} `json:"payload,omitempty" elastic_mapping:"payload:{enabled:false}"` // Store-only metadata
}

// Extend the base object with specific fields
type DataSource struct {
    CombinedFullText  // Inherit all fields from base object

    Type        string `json:"type,omitempty" elastic_mapping:"type:{type:keyword,copy_to:combined_fulltext}"`
    Name        string `json:"name" elastic_mapping:"name:{type:keyword,copy_to:combined_fulltext,fields:{text: {type: text}, pinyin: {type: text, analyzer: pinyin_analyzer}}}"`
    Description string `json:"description,omitempty" elastic_mapping:"description:{type:text,copy_to:combined_fulltext}"`
    Icon        string `json:"icon,omitempty" elastic_mapping:"icon:{enabled:false}"`
    Category    string `json:"category,omitempty" elastic_mapping:"category:{type:keyword}"`
    Tags        []string `json:"tags,omitempty" elastic_mapping:"tags:{type:keyword}"`
    Connector   ConnectorConfig `json:"connector,omitempty" elastic_mapping:"connector:{type:object}"`
    Enabled     bool   `json:"enabled" elastic_mapping:"enabled:{type:keyword}"`
}
```

### Advanced Object Definition with Elasticsearch Mapping

```go
type Document struct {
    orm.ORMObjectBase        // Embedding ORM base for persistence-related fields

    Title       string    `json:"title" elastic_mapping:"title: { type: text, analyzer: standard }"`
    Content     string    `json:"content" elastic_mapping:"content: { type: text, analyzer: standard }"`
    Tags        []string  `json:"tags" elastic_mapping:"tags: { type: keyword }"`
    Status      string    `json:"status" elastic_mapping:"status: { type: keyword }"`

    Source      DataSourceReference `json:"source"`
}
```

## Object Registration

Objects must be registered with the ORM before use. Registration typically happens during application initialization. **Note: Schema initialization is handled automatically - you only need to register your objects.**

### Simple Registration
```go
// Register object with custom index name
orm.MustRegisterSchemaWithIndexName(&User{}, "users")

// Register with default naming (struct name lowercase + 's')
orm.MustRegisterSchemaWithIndexName(&Document{}, "documents")

// Registration is complete - no additional initialization needed!
```

### Registration with Context
```go
// For advanced scenarios with sharing/multitenancy
ctx := orm.NewContext()
orm.WithModel(ctx, &User{})
orm.WithModel(ctx, &Document{})
```

## Elasticsearch Mapping with elastic_mapping

The `elastic_mapping` tag allows you to define Elasticsearch field mappings directly in your Go struct. This provides fine-grained control over how your data is indexed and queried.

### Common Mapping Parameters

| Parameter | Description | Example |
|-----------|-------------|---------|
| `type` | Field data type | `type:text`, `type:keyword`, `type:date` |
| `analyzer` | Text analyzer for indexing and searching | `analyzer:standard`, `analyzer:ik_max_word` |
| `index` | Whether field should be indexed | `index:false`, `index:true` |
| `enabled` | Enable/disable field processing | `enabled:false`, `enabled:true` |
| `store` | Store field values separately | `store:true` |
| `copy_to` | Copy field value to another field | `copy_to:combined_fulltext` |
| `fields` | Define multi-fields for different analysis | `fields:{keyword: {type: keyword}}` |
| `format` | Date format for date fields | `format:yyyy-MM-dd HH:mm:ss` |

### Advanced Mapping Options

#### Multi-field Mapping
```go
type Product struct {
    orm.ORMObjectBase

    Name string `json:"name" elastic_mapping:"name:{type:text,analyzer:standard,fields:{keyword:{type:keyword},raw:{type:keyword,index:false}}}"`
}
```

#### Object Mapping
```go
type User struct {
    orm.ORMObjectBase

    Address Address `json:"address" elastic_mapping:"address:{type:object,properties:{city:{type:keyword},country:{type:keyword}}}"`
}

// OR dynamic object mapping
type Config struct {
    orm.ORMObjectBase

    Settings map[string]interface{} `json:"settings" elastic_mapping:"settings:{type:object}"`
}
```

#### Store-only Mapping
```go
type Attachment struct {
    orm.ORMObjectBase

    FileName string `json:"filename"`
    FileData string `json:"file_data" elastic_mapping:"file_data:{enabled:false}"` // Not indexed, just stored
}
```

#### Text Analysis with Multiple Analyzers
```go
type Content struct {
    orm.ORMObjectBase

    Title string `json:"title" elastic_mapping:"title:{type:text,analyzer:standard,fields:{pinyin:{type:text,analyzer:pinyin_analyzer},keyword:{type:keyword}}}"`
}
```

### Real-World DataSource Example

From the actual codebase showing advanced mapping patterns:

```go
type CombinedFullText struct {
    orm.ORMObjectBase
    CombinedFullText  string `json:"-" elastic_mapping:"combined_fulltext:{type:text,index_prefixes:{},index_phrases:true,analyzer:combined_text_analyzer}"`

    Metadata map[string]interface{} `json:"metadata,omitempty" elastic_mapping:"metadata:{type:object}"` // Searchable metadata
    Payload  map[string]interface{} `json:"payload,omitempty" elastic_mapping:"payload:{enabled:false}"` // Store-only metadata
}

type DataSource struct {
    CombinedFullText  // Inherits all base fields

    Type        string `json:"type,omitempty" elastic_mapping:"type:{type:keyword,copy_to:combined_fulltext}"`
    Name        string `json:"name" elastic_mapping:"name:{type:keyword,copy_to:combined_fulltext,fields:{text:{type:text},pinyin:{type:text,analyzer:pinyin_analyzer}}}"`
    Description string `json:"description,omitempty" elastic_mapping:"description:{type:text,copy_to:combined_fulltext}"`
    Icon        string `json:"icon,omitempty" elastic_mapping:"icon:{enabled:false}"`
    Category    string `json:"category,omitempty" elastic_mapping:"category:{type:keyword}"`
    Tags        []string `json:"tags,omitempty" elastic_mapping:"tags:{type:keyword}"`
    Connector   ConnectorConfig `json:"connector,omitempty" elastic_mapping:"connector:{type:object}"`
    Enabled     bool `json:"enabled" elastic_mapping:"enabled:{type:keyword}"`
}
```

## CRUD Operations

### Create (Create)

```go
func createUser() {
    // Create context
    ctx := orm.NewContext()
    ctx.Refresh = orm.WaitForRefresh // Wait for index refresh

    // Create new user - ID and timestamps handled automatically by ORMObjectBase
    user := &User{
        Name:    "John Doe",
        Email:   "john@example.com",
        Age:     25,
    }

    // Insert into database
    err := orm.Create(ctx, user)
    if err != nil {
        log.Error("Failed to create user:", err)
        return
    }

    fmt.Printf("User created with ID: %s\n", user.GetID())
}
```

### Read (Get)

```go
func getUser() {
    ctx := orm.NewContext()

    user := &User{}
    user.ID = "user-id-123"

    // Get user by ID
    exists, err := orm.GetV2(ctx, user)
    if err != nil {
        log.Error("Failed to get user:", err)
        return
    }

    if !exists {
        fmt.Println("User not found")
        return
    }

    fmt.Printf("Found user: %s, Email: %s\n", user.Name, user.Email)
}

func getUserWithSystemFields() {
    ctx := orm.NewContext()

    user := &User{}
    user.ID = "user-id-123"

    // Get user including system fields
    exists, err := orm.GetWithSystemFields(ctx, user)
    if err != nil {
        log.Error("Failed to get user:", err)
        return
    }

    // Access system fields
    if exists && user.System != nil {
        ownerID := user.GetOwnerID()
        fmt.Printf("User owner ID: %s\n", ownerID)
    }
}
```

### Update (Update/Upsert)

```go
func updateUser() {
    ctx := orm.NewContext()
    ctx.Refresh = orm.WaitForRefresh

    // Get existing user - timestamps handled automatically
    user := &User{}
    user.SetID("user-id-123") // or use GetID() if you have the object

    exists, err := orm.GetV2(ctx, user)
    if err != nil || !exists {
        log.Error("User not found")
        return
    }

    // Update fields - Updated timestamp handled automatically
    user.Name = "John Smith"

    // Update in database
    err = orm.Update(ctx, user)
    if err != nil {
        log.Error("Failed to update user:", err)
        return
    }

    fmt.Println("User updated successfully")
}

func updatePartialFields() {
    ctx := orm.NewContext()
    ctx.Refresh = orm.WaitForRefresh

    // Update only specific fields
    updates := util.MapStr{
        "name":  "Johnny Doe",
        "email": "johnny@example.com",
    }

    user := &User{}
    user.ID = "user-id-123"

    err := orm.UpdatePartialFields(ctx, user, updates)
    if err != nil {
        log.Error("Failed to update user:", err)
        return
    }

    fmt.Println("User partially updated successfully")
}

func upsertUser() {
    ctx := orm.NewContext()
    ctx.Refresh = orm.WaitForRefresh

    // Create or update user - timestamps handled automatically
    user := &User{
        Name:    "John Updated",
        Email:   "john.updated@example.com",
    }

    // Use existing ID if updating, or SetID will generate one if needed
    user.SetID("user-id-123")

    err := orm.Upsert(ctx, user)
    if err != nil {
        log.Error("Failed to upsert user:", err)
        return
    }

    fmt.Println("User upserted successfully")
}
```

### Delete (Delete)

```go
func deleteUser() {
    ctx := orm.NewContext()
    ctx.Refresh = orm.WaitForRefresh

    user := &User{}
    user.SetID("user-id-123")

    // Delete user
    err := orm.Delete(ctx, user)
    if err != nil {
        log.Error("Failed to delete user:", err)
        return
    }

    fmt.Println("User deleted successfully")
}
```

## Querying with the Query Builder

The `QueryBuilder` is the backend-neutral entry point for all reads. A builder holds boolean clauses (must / should / must_not / filter), sorting, pagination, source selection, and — see the next chapter — aggregations. The same builder runs unchanged on the Elasticsearch and SQLite backends.

```go
ctx := orm.NewContext()
orm.WithModel(ctx, &Product{}) // binds the index/mapping for the backend

qb := orm.NewQuery().
    Filter(orm.TermQuery("status", "active")).  // structured filter
    Filter(orm.Range("created").Gte("2024-01-01")).
    SortBy(orm.Sort{Field: "created", SortType: orm.DESC}).
    From(0).Size(20)

res, err := orm.SearchV2(ctx, qb)
if err != nil { /* ... */ }

items, total, err := elastic.DecodeHits[Product](res) // typed decode, true ES total
fmt.Printf("total=%d, page=%d items\n", total, len(items))
```

> **Filter vs Must**: `Filter` clauses don't contribute to scoring (they are compiled to ES `filter` context). Prefer `Filter` for exact-match structured conditions and `Must` for full-text relevance.

### Term-level queries

```go
orm.TermQuery("status", "active")            // exact match (keyword/number/bool)
orm.TermsQuery("category", "a", "b", "c")    // match any of the values (set membership)
orm.InQuery("id", []interface{}{"u1", "u2"}) // set membership, idiomatic for ID lists
orm.NotInQuery("status", []interface{}{"archived", "draft"})
orm.ExistsQuery("deleted_at")                // field is present
orm.PrefixQuery("name", "infini")            // prefix match
orm.WildcardQuery("email", "*@infini.ltd")   // * and ? wildcards
orm.RegexpQuery("code", "^INF-[0-9]+")       // full regex (ES; SQLite approximates — see parity notes)
orm.FuzzyQuery("name", "infini", 2)          // edit-distance match with explicit fuzziness
```

### Full-text queries

```go
orm.MatchQuery("description", "fast search")        // analyzed match
orm.MatchPhraseQuery("title", "log pattern", 1)     // phrase with slop
orm.MultiMatchQuery([]string{"title", "body"}, "error") // OR across fields
orm.QueryStringQuery("message", "status:active AND (error OR warning)", "AND") // query-string syntax, explicit default operator
```

On SQLite, `match`/`match_phrase`/`query_string` use FTS5 when a full-text plan exists for the field and fall back to equality/LIKE otherwise (see parity notes below).

### Range queries (fluent)

```go
orm.Range("price").Gte(100)                    // >=
orm.Range("price").Lt(1000)                    // <
orm.Range("created").Gte("2024-01-01").Lte("2024-12-31") // chained: both bounds

// Build both bounds into ONE clause (recommended — a single range node):
qb.Filter(orm.MustQuery(
    orm.Range("created").Gte(start),
    orm.Range("created").Lte(end),
))
```

### Boolean composition

```go
qb := orm.NewQuery()
qb.Must(orm.MatchQuery("title", "error"))                 // scored, all must match
qb.Should(orm.MatchQuery("tags", "urgent"))               // optional boost
qb.MinimumShouldMatch(1)                                   // require at least one should
qb.Not(orm.TermQuery("status", "archived"))                // must_not
qb.Filter(orm.TermQuery("tenant", "acme"))                 // non-scoring filter

// Reusable sub-expression (grouping helpers produce one boolean clause):
cond := orm.FilterQuery(
    orm.MustQuery(orm.TermQuery("env", "prod")),
    orm.MustNotQuery(orm.ExistsQuery("deleted_at")),
)
qb.Filter(cond)
// orm.BoolQuery(orm.Filter|orm.Must|orm.Should|orm.MustNot, clauses...) is
// the explicit form of the same thing.
```

### Sorting, pagination, source selection

```go
qb.SortBy(
    orm.Sort{Field: "priority", SortType: orm.DESC},
    orm.Sort{Field: "created", SortType: orm.ASC},   // tie-break
).From(0).Size(50)

qb.Include("id", "name", "status")  // _source_includes — reduces payload
qb.Exclude("large_blob")            // _source_excludes
qb.Collapse("user_id")              // field collapse (dedupe by field, ES)
```

### Fuzziness ladder

`Fuzziness(n)` (0–5) applies progressive auto-fuzzy matching to match/multi_match text queries — useful for typo tolerance in user-typed filters:

```go
qb := orm.NewQuery().Fuzziness(2).Must(orm.MatchQuery("name", userTyped))
```

### Building from an HTTP request

`NewQueryBuilderFromRequest` wires the URL parameter contract (`query`, `filter=field:value`, `sort=field:desc`, `from`, `size`, `_source_includes`, `fuzziness`, `default_fields`, and `agg[...]` aggregations — see *Query URL Parameters* / *Aggregation via URL Parameters*) into a builder, with optional default full-text fields:

```go
builder, err := orm.NewQueryBuilderFromRequest(req, "title", "content")
if err != nil { /* 400 */ }
builder.EnableBodyBytes() // ALSO merge a raw ES DSL JSON body (ES backend only)
```

### Delete by Query

```go
func deleteOldUsers() {
    ctx := orm.NewContext()

    builder := orm.NewQuery().
        Filter(orm.Range("created").Lt("2022-01-01")) // created before 2022

    result, err := orm.DeleteByQuery(ctx, builder)
    if err != nil {
        log.Error("Delete by query failed:", err)
        return
    }
    fmt.Printf("Deleted %d old users\n", result.Deleted)
}
```

### Backend parity notes (queries)

| Capability | Elasticsearch | SQLite |
|---|---|---|
| term / terms / in / not_in / exists / prefix / wildcard / ranges | ✅ | ✅ (dates via epoch shadow columns) |
| match / phrase / multi_match / query_string | ✅ | ⚠️ FTS5 when available, else equality/LIKE; query-string operators not parsed |
| regexp / fuzzy | ✅ | ⚠️ approximated as substring LIKE |
| semantic / hybrid / nested | ✅ | ❌ compiled to a never-matching predicate (one-time warning) |
| Include / Exclude / Collapse | ✅ | ❌ currently ignored |
| Raw request-body DSL (`EnableBodyBytes`) | ✅ | ❌ ignored |

---

## Aggregations

Aggregations are a first-class operation: build an aggregation tree on a `QueryBuilder`, execute it with `orm.Aggregate`, and read back a **typed, recursive result model** — no ES-shaped JSON parsing. Bucket and metric aggregations are computed natively by each backend; pipeline aggregations are computed uniformly by the framework engine (`core/aggregate`) so every backend returns identical results.

### Execution model and result types

```go
// AggregationResult — root; keys match the names you gave in SetAggs.
// AggNode — one named agg; exactly one shape is populated:
//   Value/ValueSet  single metric (ValueSet distinguishes 0 from "no value")
//   Values          multi-value metric (e.g. percentiles)
//   TopHit          top document (top_hits)
//   Buckets         bucket list; each Bucket has Key (display), KeyRaw
//                   (epoch ms for time buckets), DocCount, and nested Aggs.
func Aggregate(ctx *Context, qb *QueryBuilder) (*AggregationResult, error)
```

Aggregations are attached with the fluent `SetAggs` (name/spec pairs, chainable) or the map form `SetAggregations`. The builder's WHERE clauses scope the aggregated document set, exactly like `SearchV2`.

### Quick start — terms + nested metric

```go
ctx := orm.NewContext()
orm.WithModel(ctx, &Order{})

avgPrice := orm.NewMetricAggregation(orm.MetricAvg, "price")
byStatus := &orm.TermsAggregation{Field: "status", Size: 10}
byStatus.AddNested("avg_price", avgPrice)      // nest a metric under each bucket

qb := orm.NewQuery().Filter(orm.Range("created").Gte("2024-01-01"))
qb.SetAggs("by_status", byStatus)

res, err := orm.Aggregate(ctx, qb)
if err != nil { /* ... */ }

for _, b := range res.Aggs["by_status"].Buckets {
    fmt.Printf("status=%s orders=%d avg_price=%.2f\n",
        b.Key, b.DocCount, b.Aggs["avg_price"].Value)
}
```

### Metric aggregations

```go
orm.NewMetricAggregation(orm.MetricSum, "bytes")
orm.NewMetricAggregation(orm.MetricMin, "latency_ms")
orm.NewMetricAggregation(orm.MetricMax, "latency_ms")
orm.NewMetricAggregation(orm.MetricCount, "id")   // value_count
orm.NewMetricAggregation(orm.MetricCardinality, "client_ip") // distinct count

// Several metrics in one pass — SetAggs takes name/spec pairs:
qb.SetAggs(
    "total_bytes", orm.NewMetricAggregation(orm.MetricSum, "bytes"),
    "p95",        &orm.PercentilesAggregation{Field: "latency_ms", Percents: []float64{50, 95, 99}},
    "worst",      &orm.TopHitsAggregation{Size: 1, Sorts: []orm.Sort{{Field: "latency_ms", SortType: orm.DESC}}},
)

r, _ := orm.Aggregate(ctx, qb)
r.Aggs["total_bytes"].Value               // float64
r.Aggs["p95"].Values["95"]                // multi-value metric
json.Unmarshal(*r.Aggs["worst"].TopHit, &order) // raw top document
```

### Bucket aggregations

```go
// terms — group by (ordered by doc_count desc, key asc tie-break; Size truncates)
cats := &orm.TermsAggregation{Field: "category", Size: 20}

// date_histogram — time buckets. Offset shifts bucket boundaries (e.g. align
// hourly buckets to "now" instead of the wall-clock hour).
now := time.Now().UTC()
hourly := &orm.DateHistogramAggregation{
    Field:    "created",
    Interval: "1h",
    Offset:   now.Sub(now.Truncate(time.Hour)), // e.g. 17m42s
}
hourly.AddNested("sum_bytes", orm.NewMetricAggregation(orm.MetricSum, "bytes"))

// filter — aggregate a subset matching a simple query
errorsOnly := &orm.FilterAggregation{Query: map[string]interface{}{
    "term": map[string]interface{}{"level": "error"},
}}

// date_range — fixed time ranges
byQuarter := &orm.DateRangeAggregation{
    Field: "created",
    Ranges: []interface{}{
        map[string]interface{}{"from": "2024-01-01", "to": "2024-04-01"},
        map[string]interface{}{"from": "2024-04-01", "to": "2024-07-01"},
    },
}

// auto_date_histogram — backend/framework picks an interval for ~N buckets
adaptive := &orm.AutoDateHistogramAggregation{Field: "created", Buckets: 30}

qb.SetAggs("cats", cats, "hourly", hourly, "errors", errorsOnly)
```

### Multi-level nesting (one SQL per bucket node on SQLite)

```go
// terms(stream) → date_histogram(1h) → sum(count): the pattern behind
// per-stream hourly trend dashboards.
streams := &orm.TermsAggregation{Field: "stream_id", Size: 1000}
trend := &orm.DateHistogramAggregation{Field: "bucket_start", Interval: "1h",
    Offset: now.Sub(now.Truncate(time.Hour))}
trend.AddNested("value", orm.NewMetricAggregation(orm.MetricSum, "count"))
streams.AddNested("trend", trend)

qb := orm.NewQuery().Filter(orm.Range("bucket_start").Gte(cutoff))
qb.SetAggs("streams", streams)

res, _ := orm.Aggregate(ctx, qb)
for _, s := range res.Aggs["streams"].Buckets {
    for _, h := range s.Aggs["trend"].Buckets {
        v := h.KeyRaw.(int64) // epoch milliseconds for time buckets
        _ = v
    }
}
```

### Pipeline aggregations

Parent pipelines (`derivative`, `bucket_script`, `bucket_sort`) are declared **inside** the bucket aggregation they derive; sibling pipelines (`sum_bucket`, `max_bucket`) sit **next to** it at the same level:

```go
dh := &orm.DateHistogramAggregation{Field: "ts", Interval: "1h"}
dh.AddNested("sum_n", orm.NewMetricAggregation(orm.MetricSum, "n"))
dh.AddNested("derivative", &orm.DerivativeAggregation{BucketsPath: "sum_n"}) // Δ per bucket
dh.AddNested("ratio", &orm.BucketScriptAggregation{                          // arithmetic
    BucketsPath: map[string]string{"a": "sum_n", "b": "doc_count"},
    Script:      "params.a / params.b",
})
dh.AddNested("top3", &orm.BucketSortAggregation{ // sort + truncate buckets
    Sort: []orm.BucketSortSpec{{Path: "sum_n", Desc: true}},
    Size: 3,
})

total := &orm.PipelineAggregation{Type: orm.MetricSumBucket, BucketsPath: "over_time>sum_n"} // total across buckets
peak := &orm.MaxBucketAggregation{BucketsPath: "over_time>sum_n"}                            // max bucket value

qb.SetAggs("over_time", dh, "total", total, "peak", peak)

res, _ := orm.Aggregate(ctx, qb)
res.Aggs["total"].Value                    // sum over all hourly buckets
res.Aggs["peak"].Value                     // max hourly sum
res.Aggs["over_time"].Buckets[0].Aggs["derivative"].ValueSet // false for the first bucket
```

`bucket_script` scripts support `+ - * / ( )` arithmetic over `params.*`; division by zero yields 0. `buckets_path` uses the two-segment `agg>metric` form. Pipelines are computed by the framework engine on **every** backend (ES native pipeline results are recomputed for parity), including zero-fill and deterministic ordering of time series.

### Backend parity notes (aggregations)

| Aggregation | Elasticsearch | SQLite |
|---|---|---|
| sum / avg / min / max / value_count / cardinality | ✅ | ✅ native SQL |
| percentiles | ✅ | ✅ exact nearest-rank |
| top_hits | ✅ | ⚠️ typed variant returns 1 document |
| median_absolute_deviation | ✅ | ❌ empty value |
| terms / date_histogram (offset) / date_range / filter / auto_date_histogram | ✅ | ✅ (interval whitelist 1m/1h/1d/1M; month ≈ 30d; terms `Include` ignored) |
| sampler | ✅ | ⚠️ full data set |
| pipelines (derivative / bucket_script / bucket_sort / sum_bucket / max_bucket) | ✅ | ✅ identical — computed by the framework engine |

## Context Options

The ORM provides various context options for controlling behavior:

```go
ctx := orm.NewContext()

// Wait for index refresh
ctx.Refresh = orm.WaitForRefresh

// Enable sharing for multi-tenant systems
ctx.Set(orm.SharingEnabled, true)
ctx.Set(orm.SharingResourceType, "users")
ctx.Set(orm.SharingCategoryCheckingChildrenEnabled, true)

// Keep system fields
ctx.Set(orm.KeepSystemFields, true)

// Model binding for type-safe operations
orm.WithModel(ctx, &User{})

// Set custom timeout
ctx.SetTimeout(30 * time.Second)
```

## Real-World Example: DataSource Module

Here's how the ORM is used in the actual codebase:

```go
func (h *APIHandler) createDatasource(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    var obj = &core.DataSource{}
    h.MustDecodeJSON(req, obj)

    // Check referenced connector
    if obj.Connector.ConnectorID == "" {
        panic("invalid connector")
    }

    ctx := orm.NewContextWithParent(req.Context())

    // Validate related object
    connector := core.Connector{}
    connector.ID = obj.Connector.ConnectorID
    exists, err := orm.GetV2(ctx, &connector)
    if !exists || err != nil {
        panic("invalid connector")
    }

    // Set refresh option and create
    ctx.Refresh = orm.WaitForRefresh
    err = orm.Create(ctx, obj)
    if err != nil {
        h.WriteError(w, err.Error(), http.StatusInternalServerError)
        return
    }

    h.WriteJSON(w, util.MapStr{
        "_id":    obj.ID,
        "result": "created",
    }, 200)
}
```