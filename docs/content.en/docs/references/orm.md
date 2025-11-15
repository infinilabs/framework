---
title: "ORM (Object-Relational Mapping)"
weight: 50
---

The INFINI Framework provides a powerful ORM system built on top of Elasticsearch(Including OpenSearch,Easysearch support), enabling developers to define, store, and query structured data objects with ease. The ORM handles object mapping, indexing, and provides a comprehensive set of CRUD operations.

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

## Advanced Operations

### Search with Query Builder

```go
func searchUsers() {
    ctx := orm.NewContext()

    // Create query builder
    builder := orm.NewQueryBuilder()

    // Add filters
    builder.Filter(orm.TermQuery("age", 25))
    builder.Filter(orm.RangeQuery("created", util.MapStr{
        "gte": "2023-01-01",
        "lte": "2023-12-31",
    }))

    // Add sorting
    builder.SortBy(orm.Sort{Field: "created", SortType: orm.DESC})

    // Execute search
    var users []User
    err, result := elastic.SearchV2WithResultItemMapper(ctx, &users, builder, nil)
    if err != nil {
        log.Error("Search failed:", err)
        return
    }

    fmt.Printf("Found %d users\n", len(users))
    for _, user := range users {
        fmt.Printf("User: %s (%s)\n", user.Name, user.Email)
    }
}
```

### Complex Search with Text Queries

```go
func searchDocuments() {
    ctx := orm.NewContext()

    builder := orm.NewQueryBuilderFromRequest(req, "title", "content")

    // Enable body bytes for receiving additional Raw QueryDSL
    builder.EnableBodyBytes()

    // Add date range filter
    builder.Filter(orm.RangeQuery("created", util.MapStr{
        "gte": "2024-01-01",
    }))

    // Add pagination
    builder.Size(20).From(0)

    // Add aggregations
    ctx.Set(orm.AggsTerms, "tags")

    var docs []Document
    err, result := elastic.SearchV2WithResultItemMapper(ctx, &docs, builder, nil)
    if err != nil {
        log.Error("Search failed:", err)
        return
    }

    // Process results
    fmt.Printf("Found %d documents\n", len(docs))
}
```

### Delete by Query

```go
func deleteOldUsers() {
    ctx := orm.NewContext()

    // Create delete query
    builder := orm.NewQueryBuilder()
    builder.Filter(orm.RangeQuery("created", util.MapStr{
        "lt": "2022-01-01", // Delete users created before 2022
    }))

    // Execute delete by query
    result, err := orm.DeleteByQuery(ctx, builder)
    if err != nil {
        log.Error("Delete by query failed:", err)
        return
    }

    fmt.Printf("Deleted %d old users\n", result.Deleted)
}
```

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