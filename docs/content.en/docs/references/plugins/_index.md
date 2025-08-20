---
title: "Plugins"
weight: 20
---

# Plugins

Plugins extend the INFINI Framework with specialized functionality for data processing, storage, filtering, and integration. Unlike modules, plugins provide specific implementations of core interfaces like filters, storage backends, and processors.

## Available Plugins

### Storage Plugins

- **[Badger](badger)** - High-performance embedded key-value storage using BadgerDB
- **[Simple KV](simple_kv)** - Simple key-value storage implementation

### Filter Plugins

- **[Bloom Filter](filter_bloom)** - Bloom filter for efficient membership testing
- **[Cuckoo Filter](filter_cuckoo)** - Cuckoo filter for approximate set membership

### Processing Plugins

- **[HTTP](http)** - HTTP client and request processing capabilities
- **[Queue](queue)** - Advanced queue processing with multiple backend support
- **[Replay](replay)** - Data replay functionality for testing and recovery

### Integration Plugins

- **[Elasticsearch](elastic)** - Elasticsearch-specific processing and indexing
- **[SMTP](smtp)** - Email notification and messaging support
- **[StatsD](stats_statsd)** - StatsD protocol statistics collection

## Plugin Architecture

Plugins in the framework implement specific interfaces:

### Storage Interface
Storage plugins implement the key-value storage interface:
```go
type Storage interface {
    Get(key string) ([]byte, error)
    Set(key string, value []byte) error
    Delete(key string) error
    Close() error
}
```

### Filter Interface
Filter plugins implement data filtering capabilities:
```go
type Filter interface {
    Add(data []byte) error
    Contains(data []byte) bool
    Reset() error
}
```

### Processor Interface
Processing plugins handle data transformation:
```go
type Processor interface {
    Process(data []byte) ([]byte, error)
    Configure(config map[string]interface{}) error
}
```

## Plugin Configuration

Plugins are configured in the main configuration file:

```yaml
plugin_name:
  enabled: true
  # plugin-specific configuration
```

## Plugin Registration

Plugins automatically register themselves during initialization:

```go
func init() {
    // Register with appropriate subsystem
    filter.Register("plugin_name", &PluginImpl{})
    kv.Register("plugin_name", &PluginImpl{})
}
```

## Common Features

Most plugins provide:

- Automatic registration with core systems
- Configuration validation
- Health monitoring integration
- Performance statistics
- Graceful shutdown support

## Development

To create a custom plugin:

1. Implement the appropriate interface (Storage, Filter, Processor, etc.)
2. Register the plugin in the `init()` function
3. Follow the naming convention for configuration
4. Provide comprehensive error handling

Example plugin structure:
```go
package myplugin

import (
    "infini.sh/framework/core/filter"
)

type MyPlugin struct {
    config *Config
}

func (p *MyPlugin) Add(data []byte) error {
    // Implementation
}

func init() {
    filter.Register("myplugin", &MyPlugin{})
}
```

## Plugin vs Module

| Aspect | Modules | Plugins |
|--------|---------|---------|
| Purpose | System-level functionality | Specialized implementations |
| Interface | Module interface | Specific interfaces (Storage, Filter, etc.) |
| Lifecycle | Full lifecycle management | Registration-based |
| Configuration | Global module config | Interface-specific config |
| Examples | API, Stats, Elasticsearch | Badger, Bloom Filter, HTTP |

See individual plugin documentation for detailed configuration and usage information.