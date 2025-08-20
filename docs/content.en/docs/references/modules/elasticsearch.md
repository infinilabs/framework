---
title: "Elasticsearch Module"
weight: 20
---

# Elasticsearch Module

The Elasticsearch module provides comprehensive integration with Elasticsearch clusters, including connection management, health monitoring, metadata refresh, and ORM capabilities for schema management.

## Features

- Multi-cluster Elasticsearch connection management
- Automatic health checking and node availability monitoring
- Metadata refresh and cluster state tracking
- ORM support with automatic schema initialization
- Template and index management
- Remote configuration support
- Dead node recovery handling

## Configuration

Configure the Elasticsearch module in your YAML configuration:

```yaml
elastic:
  elasticsearch: "default"  # Default cluster ID
  remote_configs: false
  client_timeout: "60s"
  skip_init_metadata_on_start: false
  dead_node_availability_check_interval: "30s"
  
  # Health Monitoring
  health_check:
    enabled: true
    interval: "10s"
    
  # Node Availability Checking  
  availability_check:
    enabled: true
    interval: "10s"
    
  # Metadata Refresh
  metadata_refresh:
    enabled: true
    interval: "30s"
    
  # Cluster Settings Monitoring
  cluster_settings_check:
    enabled: false
    interval: "20s"
    
  # ORM Configuration
  orm:
    enabled: false
    init_template: true
    skip_init_default_template: false
    override_exists_template: false
    build_template_for_object: false
    panic_on_init_schema_error: false
    template_name: ""
    init_schema: true
    index_prefix: ".infini_"
    index_templates: {}
    search_templates: {}
    
  # Store Configuration
  store:
    enabled: false
    index_name: ""

# Elasticsearch Cluster Configurations
elasticsearch:
  - name: "default"
    enabled: true
    endpoint: "http://localhost:9200"
    discovery:
      enabled: false
    basic_auth:
      username: "elastic"
      password: "password"
```

## Configuration Parameters

### Module Configuration (`elastic`)

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `elasticsearch` | string | `""` | Default Elasticsearch cluster ID |
| `remote_configs` | boolean | `false` | Enable remote configuration management |
| `client_timeout` | string | `"60s"` | Default client timeout |
| `skip_init_metadata_on_start` | boolean | `false` | Skip metadata initialization on startup |
| `dead_node_availability_check_interval` | string | `"30s"` | Interval for checking dead nodes |

### Health Check Configuration

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `health_check.enabled` | boolean | `true` | Enable cluster health monitoring |
| `health_check.interval` | string | `"10s"` | Health check interval |
| `availability_check.enabled` | boolean | `true` | Enable node availability monitoring |
| `availability_check.interval` | string | `"10s"` | Node availability check interval |
| `metadata_refresh.enabled` | boolean | `true` | Enable metadata refresh |
| `metadata_refresh.interval` | string | `"30s"` | Metadata refresh interval |

### ORM Configuration

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `orm.enabled` | boolean | `false` | Enable ORM functionality |
| `orm.init_template` | boolean | `true` | Initialize index templates |
| `orm.skip_init_default_template` | boolean | `false` | Skip default template initialization |
| `orm.init_schema` | boolean | `true` | Initialize database schema |
| `orm.index_prefix` | string | `".infini_"` | Prefix for system indices |

### Cluster Configuration (`elasticsearch`)

| Parameter | Type | Description |
|-----------|------|-------------|
| `name` | string | Unique cluster identifier |
| `enabled` | boolean | Enable this cluster configuration |
| `endpoint` | string | Elasticsearch cluster endpoint URL |
| `discovery.enabled` | boolean | Enable automatic node discovery |
| `basic_auth.username` | string | Basic authentication username |
| `basic_auth.password` | string | Basic authentication password |

## Features in Detail

### Health Monitoring

The module continuously monitors:
- Cluster health status (green, yellow, red)
- Individual node availability
- Dead node recovery
- Cluster settings changes

### Metadata Management

Automatically manages:
- Cluster metadata refresh
- Node discovery and topology updates
- Index and template synchronization
- Schema version tracking

### ORM Support

Provides object-relational mapping with:
- Automatic schema initialization
- Index template management
- Custom template support
- Search template management

## Usage Examples

### Basic Health Check
The module automatically exposes cluster health through the API module's health endpoint.

### Accessing Cluster Information
```go
import "infini.sh/framework/core/elastic"

// Get cluster metadata
meta := elastic.GetMetadata("default")
if meta != nil {
    fmt.Printf("Cluster health: %s\n", meta.Health.Status)
}
```

### ORM Usage
When ORM is enabled, the module automatically:
1. Creates necessary index templates
2. Initializes required indices
3. Manages schema versions
4. Handles template updates

## Integration

The Elasticsearch module integrates with:

- **API module** - Provides cluster health in system health endpoints
- **Global registry** - Registers default cluster configurations
- **Credential system** - Manages authentication credentials
- **Task system** - Schedules periodic health checks and metadata refresh
- **Event system** - Publishes cluster state changes

## Best Practices

1. **Health Monitoring**: Keep health checks enabled for production
2. **Metadata Refresh**: Adjust interval based on cluster change frequency
3. **ORM Usage**: Enable only when using framework's data persistence features
4. **Authentication**: Use proper credentials for production clusters
5. **Timeouts**: Adjust client timeout based on network conditions

## Troubleshooting

- **Connection Issues**: Check endpoint URLs and network connectivity
- **Authentication Failures**: Verify credentials and cluster security settings
- **Health Check Failures**: Review cluster status and node availability
- **Metadata Issues**: Check cluster permissions and index access rights
- **Template Errors**: Verify index template syntax and cluster version compatibility