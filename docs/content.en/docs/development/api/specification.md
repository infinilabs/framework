---
weight: 10
title: "API Specification"
---

# API Specification

This document defines the conventions and standards for building APIs with the INFINI Framework. Following these specifications ensures consistent, predictable, and maintainable API endpoints across all applications built on the framework.

## Handler Structure

Every API handler must embed the base `api.Handler` struct to gain access to all built-in request and response helpers.

```go
package mymodule

import (
    "infini.sh/framework/core/api"
    httprouter "infini.sh/framework/core/api/router"
    "net/http"
)

type APIHandler struct {
    api.Handler
}
```

## Route Registration

Routes are registered in the `init()` function of the module. The framework provides two registration functions:

- **`api.HandleUIMethod`** — registers routes for web UI clients (supports options such as permissions and features)
- **`api.HandleAPIMethod`** — registers low-level API routes without options

### Registering Routes

```go
func init() {
    handler := APIHandler{}

    api.HandleUIMethod(api.GET,    "/resources/",     handler.list,   api.RequirePermission(listPermission))
    api.HandleUIMethod(api.POST,   "/resources/",     handler.create, api.RequirePermission(createPermission))
    api.HandleUIMethod(api.GET,    "/resources/:id",  handler.get,    api.RequirePermission(readPermission))
    api.HandleUIMethod(api.PUT,    "/resources/:id",  handler.update, api.RequirePermission(updatePermission))
    api.HandleUIMethod(api.DELETE, "/resources/:id",  handler.delete, api.RequirePermission(deletePermission))
    api.HandleUIMethod(api.GET,    "/resources/_search", handler.search, api.RequirePermission(searchPermission))
}
```

### HTTP Methods

| Constant         | HTTP Verb | Typical Use                        |
|------------------|-----------|------------------------------------|
| `api.GET`        | GET       | Retrieve a resource or list        |
| `api.POST`       | POST      | Create a new resource              |
| `api.PUT`        | PUT       | Replace or update a resource       |
| `api.DELETE`     | DELETE    | Remove a resource                  |
| `api.HEAD`       | HEAD      | Retrieve headers only              |
| `api.OPTIONS`    | OPTIONS   | CORS preflight or capability query |

### Route Patterns

```go
// Static path
api.HandleUIMethod(api.GET, "/settings", handler.getSettings)

// Required path parameter
api.HandleUIMethod(api.GET, "/resources/:id", handler.get)

// Multiple path parameters
api.HandleUIMethod(api.GET, "/orgs/:orgId/repos/:repoId", handler.getRepo)

// Catch-all (optional suffix)
api.HandleUIMethod(api.GET, "/files/*filepath", handler.serveFile)
```

## Request Handling

The handler function signature is fixed for all route handlers:

```go
func (h *APIHandler) methodName(
    w http.ResponseWriter,
    req *http.Request,
    ps httprouter.Params,
) {
    // implementation
}
```

### Path Parameters

```go
// Required path parameter — panics with 400 if missing
id := ps.MustGetParameter("id")

// Optional path parameter — returns empty string if absent
id = ps.ByName("id")
```

### Query Parameters

```go
// String (returns empty string if absent)
name := h.GetParameter(req, "name")

// String with default
sort := h.GetParameterOrDefault(req, "sort", "created:desc")

// Required string — writes 400 and returns empty if absent
query := h.MustGetParameter(w, req, "q")

// Integer with default
page := h.GetIntOrDefault(req, "page", 1)
size := h.GetIntOrDefault(req, "size", 20)

// Boolean with default
active := h.GetBoolOrDefault(req, "active", true)
```

### Request Headers

```go
token := h.GetHeader(req, "Authorization", "")
contentType := h.GetHeader(req, "Content-Type", "application/json")
```

### JSON Request Body

```go
type CreateRequest struct {
    Name  string `json:"name"`
    Email string `json:"email"`
}

// Decode and return error on failure
var body CreateRequest
if err := h.DecodeJSON(req, &body); err != nil {
    h.WriteError(w, "invalid JSON body", http.StatusBadRequest)
    return
}

// Decode and panic on failure (recovery middleware catches it)
h.MustDecodeJSON(req, &body)
```

### Raw Request Body

```go
data, err := h.GetRawBody(req)
if err != nil {
    h.WriteError(w, err.Error(), http.StatusBadRequest)
    return
}
```

## Response Format

All API responses use JSON. The framework provides a set of standardized response helpers to keep responses consistent.

### Standard JSON Response

```go
h.WriteJSON(w, payload, http.StatusOK)
```

The response body can be any JSON-serializable value. For structured payloads, use `util.MapStr`:

```go
import "infini.sh/framework/core/util"

h.WriteJSON(w, util.MapStr{
    "status":  "healthy",
    "version": "1.0.0",
}, http.StatusOK)
```

### List Response

Use `WriteJSONListResult` for paginated collections. The response envelope always includes a `total` field and a `result` array:

```go
// Response: {"total": 42, "result": [...]}
h.WriteJSONListResult(w, total, items, http.StatusOK)
```

### CRUD Resource Responses

The framework provides purpose-built helpers for each CRUD operation. All of them return HTTP 200 and a JSON body with `_id` and `result` fields.

**Create**

```go
h.WriteCreatedOKJSON(w, id)
// {"_id": "<id>", "result": "created"}
```

**Update**

```go
h.WriteUpdatedOKJSON(w, id)
// {"_id": "<id>", "result": "updated"}
```

**Delete**

```go
h.WriteDeletedOKJSON(w, id)
// {"_id": "<id>", "result": "deleted"}
```

**Get (found)**

```go
h.WriteGetOKJSON(w, id, obj)
// {"found": true, "_id": "<id>", "_source": {...}}
```

**Get (not found)**

```go
h.WriteGetMissingJSON(w, id)
// {"found": false, "_id": "<id>"}   HTTP 404
```

**Acknowledgement**

```go
// Generic acknowledgement with optional extra fields
h.WriteAckJSON(w, true, http.StatusOK, nil)
// {"acknowledged": true}

// Acknowledgement with a message
h.WriteAckWithMessage(w, true, http.StatusOK, "operation completed")
// {"acknowledged": true, "message": "operation completed"}

// Shorthand for acknowledged=true, HTTP 200
h.WriteAckOKJSON(w)
```

### Response Format Reference

| Helper                        | HTTP Status | Response Body                                         |
|-------------------------------|-------------|-------------------------------------------------------|
| `WriteCreatedOKJSON(w, id)`   | 200         | `{"_id":"<id>","result":"created"}`                   |
| `WriteUpdatedOKJSON(w, id)`   | 200         | `{"_id":"<id>","result":"updated"}`                   |
| `WriteDeletedOKJSON(w, id)`   | 200         | `{"_id":"<id>","result":"deleted"}`                   |
| `WriteGetOKJSON(w, id, obj)`  | 200         | `{"found":true,"_id":"<id>","_source":{...}}`         |
| `WriteGetMissingJSON(w, id)`  | 404         | `{"found":false,"_id":"<id>"}`                        |
| `WriteAckOKJSON(w)`           | 200         | `{"acknowledged":true}`                               |
| `WriteJSONListResult(w,n,v,s)`| *status*    | `{"total":<n>,"result":[...]}`                        |
| `WriteJSON(w, v, status)`     | *status*    | *arbitrary JSON*                                      |

## Error Handling

All error responses use a standard envelope:

```json
{
  "status": 400,
  "error": {
    "reason": "<human-readable message>"
  }
}
```

Use the following helpers to produce consistent errors:

```go
// 400 Bad Request
h.Error400(w, "missing required field: name")

// 404 Not Found
h.Error404(w)

// 500 Internal Server Error
h.Error500(w, "database connection failed")
h.ErrorInternalServer(w, "unexpected error")

// Custom status code
h.WriteError(w, "conflict detected", http.StatusConflict)
```

### Panic-Based Error Handling

The framework installs a recovery middleware that intercepts panics and converts them into structured HTTP error responses. You may panic with `errors.NewWithHTTPCode` to signal specific HTTP status codes:

```go
import "infini.sh/framework/core/errors"

// Panics with a 404; recovery middleware writes the error response
panic(errors.NewWithHTTPCode(404, "resource not found"))
```

> **Note:** Only use panic-based errors for truly exceptional conditions. Prefer explicit `h.WriteError` calls in normal validation flows.

## Authentication and Authorization

### Require Login

```go
// User must be authenticated
api.HandleUIMethod(api.GET, "/profile", handler.getProfile,
    api.RequireLogin())
```

### Require Permission

```go
import "infini.sh/framework/core/security"

var readPermission = security.GetSimplePermission("category", "resource", string(security.Read))

api.HandleUIMethod(api.GET, "/resources/:id", handler.get,
    api.RequirePermission(readPermission))

// Multiple permissions — all must be satisfied
api.HandleUIMethod(api.DELETE, "/admin/resources/:id", handler.delete,
    api.RequirePermission(deletePermission, adminPermission))
```

### Optional Login

```go
// Proceeds for both authenticated and anonymous users
api.HandleUIMethod(api.GET, "/public/feed", handler.getFeed,
    api.OptionLogin())
```

### Public Access

```go
// No authentication required
api.HandleUIMethod(api.GET, "/health", handler.healthCheck,
    api.AllowPublicAccess())
```

## Route Options

Additional route-level options can be applied using functional option arguments.

### Feature Flags

```go
// Enable CORS headers for this endpoint
api.HandleUIMethod(api.GET, "/data", handler.getData,
    api.Feature(core.FeatureCORS))

// Mask sensitive fields in the response
api.HandleUIMethod(api.GET, "/users/:id", handler.getUser,
    api.Feature(core.FeatureMaskSensitiveField))

// Remove sensitive fields entirely from the response
api.HandleUIMethod(api.GET, "/users/:id", handler.getUser,
    api.Feature(core.FeatureRemoveSensitiveField))

// Enable fingerprint-based rate throttling
api.HandleUIMethod(api.POST, "/upload", handler.upload,
    api.Feature(core.FeatureFingerprintThrottle))
```

### Metadata Labels

Labels provide structured metadata used for logging, auditing, and request tracing:

```go
api.HandleUIMethod(api.POST, "/resources/", handler.create,
    api.Name("Create Resource"),
    api.Resource("resource"),
    api.Action("create"),
    api.Label("operation", "resource_management"),
    api.Tags([]string{"resource", "admin", "create"}))
```

## Complete Example

The following example demonstrates a complete CRUD API module following all the conventions above.

```go
package resource

import (
    "net/http"

    "infini.sh/framework/core/api"
    httprouter "infini.sh/framework/core/api/router"
    "infini.sh/framework/core/security"
    "infini.sh/framework/core/util"
)

type APIHandler struct {
    api.Handler
}

const category = "myapp"
const resourceType = "resource"

func init() {
    createPermission := security.GetSimplePermission(category, resourceType, string(security.Create))
    readPermission   := security.GetSimplePermission(category, resourceType, string(security.Read))
    updatePermission := security.GetSimplePermission(category, resourceType, string(security.Update))
    deletePermission := security.GetSimplePermission(category, resourceType, string(security.Delete))
    searchPermission := security.GetSimplePermission(category, resourceType, string(security.Search))

    handler := APIHandler{}

    api.HandleUIMethod(api.POST,   "/resources/",        handler.create, api.RequirePermission(createPermission))
    api.HandleUIMethod(api.GET,    "/resources/:id",     handler.get,    api.RequirePermission(readPermission))
    api.HandleUIMethod(api.PUT,    "/resources/:id",     handler.update, api.RequirePermission(updatePermission))
    api.HandleUIMethod(api.DELETE, "/resources/:id",     handler.delete, api.RequirePermission(deletePermission))
    api.HandleUIMethod(api.GET,    "/resources/_search", handler.search, api.RequirePermission(searchPermission))
}

type Resource struct {
    ID   string `json:"id"`
    Name string `json:"name"`
}

func (h *APIHandler) create(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    var obj Resource
    h.MustDecodeJSON(req, &obj)
    obj.ID = util.GetUUID()
    // ... persist obj ...
    h.WriteCreatedOKJSON(w, obj.ID)
}

func (h *APIHandler) get(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    id := ps.MustGetParameter("id")
    obj := Resource{ID: id, Name: "example"}
    // ... load obj by id ...
    h.WriteGetOKJSON(w, id, obj)
}

func (h *APIHandler) update(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    id := ps.MustGetParameter("id")
    var obj Resource
    h.MustDecodeJSON(req, &obj)
    // ... update obj by id ...
    h.WriteUpdatedOKJSON(w, id)
}

func (h *APIHandler) delete(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    id := ps.MustGetParameter("id")
    // ... delete obj by id ...
    h.WriteDeletedOKJSON(w, id)
}

func (h *APIHandler) search(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    from := h.GetIntOrDefault(req, "from", 0)
    size := h.GetIntOrDefault(req, "size", 10)
    // Replace the lines below with your own data-access logic.
    items := make([]Resource, 0)
    var total int64
    // Example: items, total = store.Query(from, size)
    h.WriteJSONListResult(w, total, items, http.StatusOK)
}
```
