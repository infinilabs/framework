---
title: "API Module"
weight: 10
---

# API Module

The API module provides REST API endpoints for system information, health checks, and application settings. It serves as the core HTTP interface for interacting with the framework.

## Features

- System information endpoints
- Health check monitoring
- Application settings management
- Logging configuration management
- Auto-discovery API directory

## Configuration

The API module uses the global `api` configuration section. Here are the key configuration options:

```yaml
api:
  enabled: true
  network:
    bind: "0.0.0.0:8080"
    publish: "localhost:8080"
  disable_api_directory: false
  api_directory_path: "/"
```

## API Endpoints

The API module provides the following built-in endpoints:

### System Information

- **GET `/_whoami`** - Returns the publish address of the API server
- **GET `/_version`** - Returns the application version
- **GET `/_info`** - Returns detailed system information including:
  - Host information (OS, architecture)
  - Hardware information (CPU cores, model)
  - Application build details

### Health Monitoring

- **GET `/health`** - Returns system health status including:
  - Overall system status
  - Service health status
  - Elasticsearch cluster health (if configured)
  - Setup requirements status

Example health response:
```json
{
  "status": "green",
  "services": {
    "system_cluster": "green"
  }
}
```

### Settings Management

- **GET/POST `/setting/logger`** - Manage logging configuration
- **GET `/setting/application`** - Retrieve application settings

### API Directory

- **GET `/`** (configurable) - Returns a directory of available API endpoints

## Module Structure

The API module consists of:

- **APIModule** - Main module struct implementing the Module interface
- **Handler methods** - Individual endpoint handlers for system operations
- **Settings management** - Configuration and logging settings endpoints

## Usage Example

The API module is automatically enabled and provides immediate access to system endpoints:

```bash
# Check system health
curl http://localhost:8080/health

# Get system information
curl http://localhost:8080/_info

# Check API version
curl http://localhost:8080/_version
```

## Integration

The API module integrates with:

- **Global environment** - For system configuration and health status
- **Elasticsearch module** - For cluster health monitoring
- **Logging system** - For runtime logging configuration
- **Host utilities** - For system information gathering

## Security

- Some endpoints require authentication when UI module is enabled
- Health endpoints are generally publicly accessible
- Settings endpoints may require elevated privileges