---
title: "API & Web Framework"
weight: 50
---

# API & Web Framework

The INFINI Framework provides a comprehensive API and Web framework built on top of Go's HTTP ecosystem, featuring high-performance routing, flexible middleware system, and built-in security features.

## Overview

The framework offers two separate but complementary HTTP servers:
- **API Server**: Designed for programmatic access and RESTful APIs
- **Web Server**: Designed for web interfaces and UI applications

Both servers share common patterns and can be configured independently.

## Architecture

### Core Components

- **HTTP Router**: High-performance trie-based routing with path parameters
- **Handler System**: Structured request processing with built-in utilities
- **Middleware**: Flexible filter chain for cross-cutting concerns
- **Security**: Integrated authentication, authorization, and CORS support
- **Response Helpers**: Standardized JSON responses and error handling

## Configuration

### API Server Configuration

```yaml
api:
  enabled: true
  network:
    host: "0.0.0.0"
    port: 9200
    binding: "0.0.0.0:9200"
    publish: "localhost:9200"
    skip_occupied_port: true
  security:
    enabled: true
    username: "admin"
    password: "password"
  cors:
    allowed_origins:
      - "https://domain.com"
      - "http://localhost:3000"
  websocket:
    enabled: true
    base_path: "/ws"
    permitted_hosts: []
    skip_host_verify: false
  base_path: "/_api"      # Base path for API endpoints
  verbose_error_root_cause: false
  api_directory_path: "/api"  # API documentation directory
  disable_api_directory: false
```

### Web Server Configuration

```yaml
web:
  enabled: true
  network:
    host: "0.0.0.0"
    port: 8090
    binding: "0.0.0.0:8090"
    publish: "localhost:8090"
    skip_occupied_port: true
  security:
    enabled: true
    authentication:
      native:
        enabled: true
      # Note: http_basic authentication is not supported yet for web
      # users should use API basic auth or OAuth2 instead
      http_basic:
        enabled: false
        # endpoint: "https://auth.example.com"      # Not implemented
        # secret_id: ""
        # secret_key: ""
  cors:
    allowed_origins:
      - "https://domain.com"
  websocket:
    enabled: true
    base_path: "/ws"
    permitted_hosts: []
    skip_host_verify: false
  base_path: ""            # Base path for web application
  gzip:
    enabled: true
    level: 6
  cookie:
    store: "cookie"
    secure: true
    httponly: true
    max_age: 2592000
    path: "/"
  ui:
    local:
      enabled: true
      path: "./web/dist"
    vfs:
      enabled: false
  embedding_api: false
```

## Handler Definition

### Basic Handler Structure

Create a handler by embedding the base `api.Handler`:

```go
package mymodule

import (
    "infini.sh/framework/core/api"
    httprouter "infini.sh/framework/core/api/router"
    "net/http"
)

type APIHandler struct {
    api.Handler  // Embedding base handler for all functionality
}
```

### Handler Registration

Register handlers using the built-in routing methods:

```go
func init() {
    // Create handler instance
    handler := APIHandler{}

    // Register with permissions and features
    api.HandleUIMethod(api.POST, "/users/", handler.createUser,
        api.RequirePermission(createPermission))
    api.HandleUIMethod(api.GET, "/users/:id", handler.getUser,
        api.RequirePermission(readPermission))
    api.HandleUIMethod(api.PUT, "/users/:id", handler.updateUser,
        api.RequirePermission(updatePermission))
    api.HandleUIMethod(api.DELETE, "/users/:id", handler.deleteUser,
        api.RequirePermission(deletePermission))
    api.HandleUIMethod(api.GET, "/users/_search", handler.searchUsers,
        api.RequirePermission(searchPermission))
}
```

## Route Patterns and Parameters

### Basic Routes

```go
// Static paths
api.HandleUIMethod(api.GET, "/settings", handler.getSettings)
api.HandleUIMethod(api.POST, "/settings", handler.updateSettings)

// Path parameters (required)
api.HandleUIMethod(api.GET, "/users/:id", handler.getUser)
api.HandleUIMethod(api.GET, "/documents/:docId/comments/:commentId", handler.getComment)

// Catch-all parameters (optional)
api.HandleUIMethod(api.GET, "/files/*filepath", handler.serveFile)
```

### Parameter Extraction

```go
func (h *APIHandler) getUser(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    // Extract path parameter
    userID := ps.MustGetParameter("id")

    // Alternative extraction
    userID = ps.ByName("id")

    // Query parameters
    page := h.GetIntOrDefault(req, "page", 1)
    size := h.GetIntOrDefault(req, "size", 20)

    // Headers
    authToken := h.GetHeader(req, "Authorization", "")
}
```

## Request Handling

### JSON Input Processing

```go
func (h *APIHandler) createUser(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    var user CreateUserRequest

    // Validate and decode JSON input
    err := h.DecodeJSON(req, &user)
    if err != nil {
        h.WriteError(w, "Invalid JSON input", http.StatusBadRequest)
        return
    }

    // OR use MustDecodeJSON for automatic error handling
    h.MustDecodeJSON(req, &user)
}

func (h *APIHandler) updateUser(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    var updates map[string]interface{}

    // Use for partial updates
    err := h.DecodeJSON(req, &updates)
    if err != nil {
        h.WriteJSON(w, util.MapStr{"error": "Invalid input"}, http.StatusBadRequest)
        return
    }
}
```

### Query Parameter Processing

```go
func (h *APIHandler) searchUsers(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    // Required parameters
    query := h.MustGetParameter(w, req, "q")

    // Optional parameters with defaults
    page := h.GetIntOrDefault(req, "page", 1)
    size := h.GetIntOrDefault(req, "size", 10)
    sort := h.GetParameterOrDefault(req, "sort", "created:desc")

    // Boolean parameters
    activeOnly := h.GetBoolOrDefault(req, "active", true)

    // Multiple values
    status := req.URL.Query()["status"]
    tags := req.URL.Query()["tags"]
}
```

## Response Helpers

### Standard JSON Responses

```go
// Simple JSON response
func (h *APIHandler) getStatus(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    status := map[string]interface{}{
        "status": "healthy",
        "version": "1.0.0",
    }
    h.WriteJSON(w, status, http.StatusOK)
}

// List with total count
func (h *APIHandler) listUsers(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    users := []User{{}, {}}
    h.WriteJSONListResult(w, 2, users, http.StatusOK)
}

// Acknowledgment response
func (h *APIHandler) deleteUser(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    h.WriteAckJSON(w, true, http.StatusOK, nil)
}

// Creation response
func (h *APIHandler) createUser(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    h.WriteCreatedOKJSON(w, "user-123")
}

// Update response
func (h *APIHandler) updateUser(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    h.WriteUpdatedOKJSON(w, "user-123")
}
```

### Error Handling

```go
// Standard error response
func (h *APIHandler) handleError(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    h.WriteError(w, "Resource not found", http.StatusNotFound)
}

// Detailed error object
func (h *APIHandler) handleComplexError(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    err := util.MapStr{
        "error": "Validation failed",
        "field_errors": map[string]string{
            "email": "invalid format",
            "age": "must be positive",
        },
    }
    h.WriteErrorObject(w, err, http.StatusBadRequest)
}

// HTTP status code errors
func (h *APIHandler) handleVariousErrors(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    h.Error404(w)      // 404 Not Found
    h.Error400(w, "Bad request")  // 400 Bad Request
    h.Error500(w, "Internal server error")  // 500 Internal Server Error
    h.ErrorInternalServer(w, "Something went wrong")  // 500, alternative
}
```

## API Middleware and Options

### Permission-based Access Control

```go
import "infini.sh/framework/core/security"

// Define permissions
var createPermission = security.GetSimplePermission("user", "user", string(security.Create))
var readPermission = security.GetSimplePermission("user", "user", string(security.Read))
var updatePermission = security.GetSimplePermission("user", "user", string(security.Update))
var deletePermission = security.GetSimplePermission("user", "user", string(security.Delete))

// Register with permissions
api.HandleUIMethod(api.POST, "/users/", handler.createUser, api.RequirePermission(createPermission))
api.HandleUIMethod(api.GET, "/users/:id", handler.getUser, api.RequirePermission(readPermission))

// Multiple permissions - require all
api.HandleUIMethod(api.DELETE, "/admin/users/:id", handler.deleteUser,
    api.RequirePermission(deletePermission, adminPermission))
```

### Feature Flags

```go
// Enable CORS for specific endpoints
api.HandleUIMethod(api.OPTIONS, "/users/_search", handler.searchUsers,
    api.Feature(core.FeatureCORS))

// Note: Request logging is not yet available as a feature flag
// Use manual logging in handlers if needed

// Disable sensitive field exposure
api.HandleUIMethod(api.GET, "/users/:id", handler.getUser,
    api.Feature(core.FeatureMaskSensitiveField))

// Enable JSON schema validation
api.HandleUIMethod(api.POST, "/users/", handler.createUser,
    api.Feature(core.FeatureValidateRequestJSON))
```

### Authentication Options

The framework supports multiple authentication backends.

```go
// Require login with standard auth
api.HandleUIMethod(api.POST, "/users/", handler.createUser,
    api.RequireLogin())

// Optional login - process if authenticated, allow anonymous otherwise
api.HandleUIMethod(api.GET, "/public/users/", handler.listPublicUsers,
    api.OptionLogin())

// Public access - no authentication required
api.HandleUIMethod(api.GET, "/health", handler.healthCheck,
    api.AllowPublicAccess())
```

### Label and Metadata

```go
// Add metadata for logging or processing
api.HandleUIMethod(api.POST, "/users/", handler.createUser,
    api.Name("Create User"),
    api.Resource("user"),
    api.Action("create"),
    api.Label("operation", "user_management"),
    api.Label("tag", "admin"),
    api.Tags([]string{"user", "admin", "create"}))
```

### Available Features Reference

| Feature | Description | Usage |
|---------|-------------|-------|
| `core.FeatureCORS` | Enable CORS headers for cross-origin requests | Standard CORS support |
| `core.FeatureNotAllowCredentials` | Disable credentials in CORS responses | CORS with no credentials |
| `core.FeatureByPassCORSCheck` | Skip CORS checking entirely | Internal endpoints |
| `core.FeatureFingerprintThrottle` | Enable fingerprint-based rate throttling | Rate limiting |
| `core.FeatureMaskSensitiveField` | Mask sensitive field values in responses | Hide data partially |
| `core.FeatureRemoveSensitiveField` | Remove sensitive fields from responses | Hide data completely |

## Web Framework Features

The web framework is primarily designed for serving static UI assets and handling web interfaces. Key features include:

### Static File Serving with VFS

Static files are managed through the Virtual File System (VFS) which provides both embedded and local file serving capabilities with runtime customization support:

```go
// Registration in main.go
vfs.RegisterFS(public.StaticFS{
    StaticFolder: global.Env().SystemConfig.WebAppConfig.UI.LocalPath,
    TrimLeftPath: global.Env().SystemConfig.WebAppConfig.UI.LocalPath,
    CheckLocalFirst: global.Env().SystemConfig.WebAppConfig.UI.LocalEnabled,
    SkipVFS: !global.Env().SystemConfig.WebAppConfig.UI.VFSEnabled,
})

// Then register the file server
api1.HandleUI("/", vfs.FileServer(vfs.VFS()))
```

### VFS File Lookup Strategy

The VFS system uses a "local-first" lookup strategy:

1. **Check local files** in the configured folder (default: `.public` relative to app binary)
2. **Fall back to embedded files** if local file doesn't exist
3. **This allows runtime customization** by replacing files in the local folder

### Configuration Example

```yaml
web:
  ui:
    local:
      enabled: true      # Enable local file checking
      path: "./web/dist" # Path to local files (relative to binary)
    vfs:
      enabled: false     # Use embedded files when local disabled
```

### Customization Workflow

1. Place your static files in the local folder (default: `.public/` relative to app binary location)
2. Files in this folder will override embedded files with the same path
3. Changes take effect immediately without restart
4. Perfect for theme customization, branding, or emergency fixes

### Configuration-Only Features

Unlike the API server, web framework features are primarily configuration-driven:

```yaml
web:
  gzip:
    enabled: true    # Enable gzip compression
    level: 6        # Compression level 1-9

  cookie:
    secure: true    # HTTPS only cookies
    httponly: true  # Prevent XSS attacks
    path: "/"       # Cookie scope

  cors:
    allowed_origins: ["https://example.com"]  # CORS domains
```


### WebSocket Support

```yaml
api:
  websocket:
    enabled: true
    base_path: "/ws"
    permitted_hosts: []
    skip_host_verify: false
```

```go
func (h *APIHandler) handleWebSocket(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    roomID := ps.ByName("room_id")

    // Upgrade HTTP to WebSocket
    conn, err := websocket.Upgrade(w, req)
    if err != nil {
        h.WriteError(w, "WebSocket upgrade failed", http.StatusBadRequest)
        return
    }

    // Join chat room
    room := websocket.GetRoom(roomID)
    room.Join(conn)
}
```

### Session Management

```go
func (h *APIHandler) login(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    var login LoginRequest
    h.MustDecodeJSON(req, &login)

    // Validate credentials
    user, err := authService.Login(login.Username, login.Password)
    if err != nil {
        h.WriteError(w, "Invalid credentials", http.StatusUnauthorized)
        return
    }

    // Create session
    session := h.CreateSession(w, user.ID)
    session.Set("user", user)
    session.Set("last_login", time.Now())

    h.WriteJSON(w, map[string]interface{}{
        "message": "Login successful",
        "user": user,
    }, http.StatusOK)
}
```

## API Handler Base Functions

The embedded `api.Handler` provides comprehensive helper methods:

### HTTP Method Handling
- `WriteJSON(w, v, code)` - Standard JSON response
- `WriteJSONListResult(w, total, v, code)` - List with total count
- `WriteError(w, message, code)` - Error response
- `WriteAckJSON(w, ack, code, obj)` - Acknowledgment response
- `WriteCreatedOKJSON(w, id)` - Creation confirmation
- `WriteUpdatedOKJSON(w, id)` - Update confirmation
- `WriteDeletedOKJSON(w, id)` - Deletion confirmation

### Parameter Processing
- `GetParameter(r, key)` - Get query parameter
- `GetParameterOrDefault(r, key, def)` - Get parameter with default
- `MustGetParameter(w, r, key)` - Required parameter (404 if missing)
- `GetIntOrDefault(r, key, def)` - Integer parameter
- `GetBoolOrDefault(r, key, def)` - Boolean parameter
- `GetHeader(r, header, def)` - HTTP header access

### JSON Processing
- `MustDecodeJSON(r, obj)` - Decode JSON with automatic error handling
- `DecodeJSON(r, obj)` - Decode JSON returning error
- `GetJSON(r)` - Get JSONQ wrapper for complex parsing
- `GetRawBody(r)` - Get raw request body as bytes

### Error Responses
- `Error500(w, msg)` - Internal server error
- `Error400(w, msg)` - Bad request
- `Error404(w)` - Not found
- `WriteErrorObject(w, err, code)` - Complex error structure

## Real-World Example: DataSource Module

Complete API implementation from the framework:

```go
package datasource

import (
    "infini.sh/framework/core/api"
    httprouter "infini.sh/framework/core/api/router"
    "infini.sh/framework/core/security"
    "infini.sh/framework/core/util"
    "net/http"
)

type APIHandler struct {
    api.Handler
}

const Category = "coco"
const Datasource = "datasource"

func init() {
    // Define permissions
    createPermission := security.GetSimplePermission(Category, Datasource, string(security.Create))
    updatePermission := security.GetSimplePermission(Category, Datasource, string(security.Update))
    readPermission := security.GetSimplePermission(Category, Datasource, string(security.Read))
    deletePermission := security.GetSimplePermission(Category, Datasource, string(security.Delete))
    searchPermission := security.GetSimplePermission(Category, Datasource, string(security.Search))

    // Register permissions
    security.RegisterPermissionsToRole(core.WidgetRole, searchPermission)

    handler := APIHandler{}

    // Register routes with permissions and features
    api.HandleUIMethod(api.POST, "/datasource/", handler.createDatasource,
        api.RequirePermission(createPermission))

    api.HandleUIMethod(api.GET, "/datasource/:id", handler.getDatasource,
        api.RequirePermission(readPermission))

    api.HandleUIMethod(api.PUT, "/datasource/:id", handler.updateDatasource,
        api.RequirePermission(updatePermission))

    api.HandleUIMethod(api.DELETE, "/datasource/:id", handler.deleteDatasource,
        api.RequirePermission(deletePermission))

    api.HandleUIMethod(api.GET, "/datasource/_search", handler.searchDatasource,
        api.RequirePermission(searchPermission),
        api.Feature(core.FeatureCORS),
        api.Feature(core.FeatureMaskSensitiveField),
        api.Label(core.SensitiveFields, secretKeys))
}

func (h *APIHandler) createDatasource(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    var obj = &core.DataSource{}
    h.MustDecodeJSON(req, obj)

    // Input validation
    if obj.Connector.ConnectorID == "" {
        panic("invalid connector")
    }

    // Create with ORM
    err := orm.Create(ctx, obj)
    if err != nil {
        h.WriteError(w, err.Error(), http.StatusInternalServerError)
        return
    }

    h.WriteCreatedOKJSON(w, obj.ID)
}

func (h *APIHandler) getDatasource(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    id := ps.MustGetParameter("id")

    obj := core.DataSource{}
    obj.ID = id

    exists, err := orm.GetV2(ctx, &obj)
    if err != nil {
        h.WriteError(w, err.Error(), http.StatusInternalServerError)
        return
    }

    if !exists {
        h.WriteGetMissingJSON(w, id)
        return
    }

    h.WriteGetOKJSON(w, id, obj)
}
```

## Best Practices

### 1. Consistent Response Patterns
```go
// Always use framework helpers for consistent responses
func GoodExample(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    user := getUser(id)
    h.WriteGetOKJSON(w, id, user)
}

func BadExample(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    user := getUser(id)
    h.WriteJSON(w, map[string]interface{}{
        "_id": id,
        "found": true,
        "_source": user,
    }, http.StatusOK)
}
```

### 2. Proper Error Handling
```go
func GoodErrorHandling(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    if err != nil {
        // Use appropriate HTTP status codes
        if err == ErrNotFound {
            h.WriteError(w, "User not found", http.StatusNotFound)
            return
        }
        if err == ErrValidation {
            h.WriteError(w, "Invalid input", http.StatusBadRequest)
            return
        }
        h.WriteError(w, "Internal error", http.StatusInternalServerError)
        return
    }
}
```

### 3. Parameter Validation
```go
func ProperValidation(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    // Extract parameters
    id := ps.MustGetParameter("id")

    // Validate required parameters
    if id == "" {
        h.WriteError(w, "ID parameter is required", http.StatusBadRequest)
        return
    }

    // Validate optional parameters
    page := h.GetIntOrDefault(req, "page", 1)
    if page < 1 {
        h.WriteError(w, "Page must be positive", http.StatusBadRequest)
        return
    }
}
```

### 4. Security Integration
```go
func SecureEndpoint(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    handler := APIHandler{}

    api.HandleUIMethod(api.POST, "/admin/update", handler.updateAdmin,
        api.RequireLogin(),           // Require authentication
        api.RequirePermission(adminPermission),  // Require specific permission
        api.Feature(core.FeatureCORS)) // Enable CORS if needed
}
```

The INFINI API/Web framework provides a solid foundation for building scalable, secure, and well-structured HTTP services with minimal boilerplate while maintaining flexibility for complex applications.