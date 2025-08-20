---
title: "Modules"
weight: 10
---

# Modules

Modules are core components of the INFINI Framework that provide essential system-level functionality. Each module implements the standard Module interface with Setup, Start, Stop, and Name methods.

## Available Modules

### Core System Modules

- **[API](api)** - REST API endpoints for system information, health checks, and settings management
- **[Elasticsearch](elasticsearch)** - Elasticsearch cluster integration with health monitoring and ORM support  
- **[Stats](stats)** - Statistics collection, monitoring, and Prometheus metrics export

### Data & Storage Modules

- **[Keystore](keystore)** - Secure key-value storage for sensitive configuration data
- **[Queue](queue)** - Message queue processing with disk and memory backends
- **[Redis](redis)** - Redis integration for caching and data storage

### Processing Modules

- **[Pipeline](pipeline)** - Data processing pipeline for ETL operations
- **[Task](task)** - Task scheduling and execution management
- **[Metrics](metrics)** - Advanced metrics collection and reporting

### Integration Modules

- **[S3](s3)** - AWS S3 integration for object storage
- **[Web](web)** - Web interface and WebSocket support
- **[Configs](configs)** - Configuration management with remote config support

## Module Configuration

All modules follow a standard configuration pattern:

```yaml
module_name:
  enabled: true
  # module-specific configuration options
```

## Module Lifecycle

Each module follows the standard lifecycle:

1. **Setup** - Initialize configuration and register handlers
2. **Start** - Begin module operations and background tasks  
3. **Stop** - Clean shutdown and resource cleanup

## Common Features

Most modules provide:

- Configuration validation and defaults
- Health check integration
- API endpoint registration
- Graceful shutdown handling
- Error handling and logging

## Development

To create a custom module, implement the Module interface:

```go
type Module interface {
    Setup()
    Start() error
    Stop() error
    Name() string
}
```

See the [development documentation](../../development/) for more details on module development.