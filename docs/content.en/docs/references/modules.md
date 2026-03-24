---
title: "Module System"
weight: 10
---
# Module System

The INFINI Framework uses a modular architecture where functionality is organized into self-contained modules. Each module follows a standard lifecycle and can be registered, configured, enabled, or disabled independently. The module system manages startup ordering through priorities and provides a clean separation between core system modules and user-provided plugins.

## Module Interface

Every module must implement the `Module` interface defined in `core/module/interface.go`:

```go
type Module interface {
    Setup()
    Start() error
    Stop() error
    Name() string
}
```

### Methods

| Method | Description |
|--------|-------------|
| `Name() string` | Returns a unique identifier for the module, used in configuration and logging. |
| `Setup()` | Called during the initialization phase. Use this to parse configuration, register API routes, register with subsystems (e.g., KV stores, filters), and perform any pre-start wiring. |
| `Start() error` | Called after all modules have been set up. Use this to start background goroutines, open connections, and begin processing. Return an error to signal a startup failure. |
| `Stop() error` | Called during shutdown. Use this to close connections, flush buffers, and release resources. Return an error to signal a shutdown problem. |

## Registration Functions

Modules register themselves using one of the registration functions from `core/module/module.go`. Registration typically happens inside a Go `init()` function so that importing the module's package is sufficient to register it.

```go
// Register a core system module (default priority).
module.RegisterSystemModule(mod Module)

// Register a system module with explicit priority control.
module.RegisterModuleWithPriority(mod Module, priority int)

// Register a user/optional plugin (default priority).
module.RegisterUserPlugin(mod Module)

// Register a user plugin with explicit priority control.
module.RegisterPluginWithPriority(mod Module, priority int)
```

### System Modules vs. User Plugins

| Category | Function | Typical Use |
|----------|----------|-------------|
| **System Module** | `RegisterSystemModule` / `RegisterModuleWithPriority` | Core framework services such as the API server, task scheduler, or pipeline engine. These are started first. |
| **User Plugin** | `RegisterUserPlugin` / `RegisterPluginWithPriority` | Optional or application-specific functionality such as storage backends, metric exporters, or security modules. These are started after all system modules. |

### Registration Pattern

The idiomatic way to register a module is via Go's `init()` function:

```go
func init() {
    module.RegisterModuleWithPriority(&MyModule{}, 0)
}
```

Importing the package that contains this `init()` function is all that is needed to register the module with the framework.

## Lifecycle

The framework manages all registered modules through a three-phase lifecycle:

```
┌─────────────────────────────────────────────────────┐
│  1. SETUP   (all system modules, then user plugins) │
│     Sorted by priority (ascending). Each module's   │
│     Setup() is called in order.                     │
├─────────────────────────────────────────────────────┤
│  2. START   (all system modules, then user plugins) │
│     Same sorted order. Each module's Start() is     │
│     called. Errors abort the startup.               │
├─────────────────────────────────────────────────────┤
│  3. STOP    (reverse order)                         │
│     On shutdown, Stop() is called on every started  │
│     module in reverse order so that dependencies    │
│     are torn down correctly.                        │
└─────────────────────────────────────────────────────┘
```

### Priority System

Priority is an integer value that controls the order in which modules are initialized and started. **Lower values run first.** Negative priorities are commonly used for foundational services that other modules depend on.

| Priority | Typical Use |
|----------|-------------|
| `-100` | Low-level storage backends (e.g., Badger KV store) |
| `0` | Default — most modules |
| `> 0` | Modules that depend on other modules being ready |

Example — registering a storage backend early:

```go
func init() {
    module.RegisterModuleWithPriority(&Module{}, -100)
}
```

## Configuration

Modules load their configuration from the application's YAML config file using `env.ParseConfig`. The first argument is the YAML section name, and the second is a pointer to a configuration struct. Struct fields are mapped to YAML keys via `config` tags.

### Defining a Config Struct

```go
type Config struct {
    Enabled       bool   `config:"enabled"`
    ListenAddress string `config:"listen_address"`
    MaxRetries    int    `config:"max_retries"`
}
```

### Parsing Config in Setup

```go
func (module *MyModule) Setup() {
    module.cfg = &Config{
        Enabled:       true,           // default value
        ListenAddress: "0.0.0.0:8080", // default value
        MaxRetries:    3,              // default value
    }
    env.ParseConfig("my_module", module.cfg)
}
```

The corresponding YAML section:

```yaml
my_module:
  enabled: true
  listen_address: "0.0.0.0:9200"
  max_retries: 5
```

You can also parse configuration directly into the module struct if it carries `config` tags:

```go
type TaskModule struct {
    TimeZone                string `config:"time_zone"`
    MaxConcurrentNumOfTasks int    `config:"max_concurrent_tasks"`
}

func (module *TaskModule) Setup() {
    module.TimeZone = "UTC"                // default
    module.MaxConcurrentNumOfTasks = 100   // default
    env.ParseConfig("task", module)
}
```

## Enabling and Disabling Modules

Modules commonly check an `Enabled` field in their configuration to decide whether to activate. This allows operators to toggle functionality without removing code:

```go
func (module *Module) Start() error {
    if !module.cfg.Enabled {
        return nil
    }
    return module.Open()
}

func (module *Module) Stop() error {
    if !module.cfg.Enabled {
        return nil
    }
    return module.Close()
}
```

In the YAML configuration file:

```yaml
my_module:
  enabled: false
```

Setting `enabled: false` causes the module to skip its initialization logic while still being registered with the framework.

## Complete Example

Below is a complete, working example of a custom module that exposes an HTTP health-check endpoint and reads its configuration from YAML.

### Module Code

```go
package health

import (
    "net/http"

    httprouter "infini.sh/framework/core/api/router"
    "infini.sh/framework/core/api"
    "infini.sh/framework/core/env"
    "infini.sh/framework/core/module"
)

// Config holds the module's YAML configuration.
type Config struct {
    Enabled bool   `config:"enabled"`
    Path    string `config:"path"`
}

// HealthModule provides a simple health-check endpoint.
type HealthModule struct {
    cfg *Config
    api.Handler
}

func (m *HealthModule) Name() string {
    return "health"
}

func (m *HealthModule) Setup() {
    m.cfg = &Config{
        Enabled: true,
        Path:    "/_health",
    }
    env.ParseConfig("health", m.cfg)

    if m.cfg.Enabled {
        api.HandleAPIMethod(api.GET, m.cfg.Path, m.handleHealth)
    }
}

func (m *HealthModule) Start() error {
    if !m.cfg.Enabled {
        return nil
    }
    // Additional start logic (background tasks, connections, etc.)
    return nil
}

func (m *HealthModule) Stop() error {
    // Cleanup resources if needed.
    return nil
}

func (m *HealthModule) handleHealth(w http.ResponseWriter, req *http.Request, ps httprouter.Params) {
    m.WriteJSON(w, map[string]string{"status": "ok"}, http.StatusOK)
}

// Register the module when this package is imported.
func init() {
    module.RegisterUserPlugin(&HealthModule{})
}
```

### YAML Configuration

```yaml
health:
  enabled: true
  path: "/_health"
```

### Importing the Module

In your application's main package or plugin registry, import the module so its `init()` function runs:

```go
import _ "your_app/plugins/health"
```

## Built-in Modules

The framework ships with a set of built-in system modules and plugins.

### System Modules (`modules/`)

| Module | Description |
|--------|-------------|
| `api` | HTTP API server and route registration |
| `configs` | Configuration management |
| `elastic` | Elasticsearch / OpenSearch / Easysearch integration |
| `keystore` | Secure credential and secret storage |
| `metrics` | Metrics collection and reporting |
| `pipeline` | Data processing pipeline engine |
| `queue` | Message queue management |
| `redis` | Redis client integration |
| `s3` | S3-compatible object storage |
| `security` | Authentication and authorization (RBAC, OAuth) |
| `stats` | Internal statistics tracking |
| `task` | Scheduled and on-demand task execution |
| `web` | Static file serving and web UI |

### Plugins (`plugins/`)

| Plugin | Description |
|--------|-------------|
| `badger` | Embedded key-value store (BadgerDB) |
| `elastic` | Elasticsearch-specific plugin extensions |
| `filter_bloom` | Bloom filter implementation |
| `filter_cuckoo` | Cuckoo filter implementation |
| `http` | HTTP pipeline processor |
| `queue` | Queue backend implementations |
| `replay` | Event replay functionality |
| `simple_kv` | Simple in-memory key-value store |
| `smtp` | SMTP email sending |
| `stats_statsd` | StatsD metrics exporter |
