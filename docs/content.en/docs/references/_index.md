---
title: "References"
weight: 20
bookCollapseSection: true
---

# Reference Documentation

This section contains comprehensive reference documentation for the INFINI Framework, including modules, plugins, configuration options, APIs, and tools.

## Component Documentation

### [Modules](modules/)
Core system modules that provide essential framework functionality:

- **[API](modules/api)** - REST API endpoints and system information
- **[Elasticsearch](modules/elasticsearch)** - Elasticsearch cluster integration and management
- **[Stats](modules/stats)** - Statistics collection and Prometheus metrics
- **[Keystore](modules/keystore)** - Secure key-value storage for sensitive data
- **[Web](modules/web)** - Web interface and WebSocket support

### [Plugins](plugins/)
Specialized plugins that extend framework capabilities:

- **[Badger](plugins/badger)** - High-performance embedded key-value storage
- **[HTTP](plugins/http)** - HTTP client and request processing
- **[Simple KV](plugins/simple_kv)** - Lightweight file-based key-value storage
- **[SMTP](plugins/smtp)** - Email notification and messaging

## Configuration References

- [HTTP Client](http_client) - HTTP client configuration and proxy settings
- [Makefile](makefile) - Build system reference and development tools
- [Query URL](query_url) - URL query parameter handling

## Architecture Overview

The INFINI Framework is built on a modular architecture:

- **Modules** provide core system functionality with full lifecycle management
- **Plugins** offer specialized implementations of interfaces (storage, filters, processors)
- **Configuration** uses YAML-based configuration with environment variable support
- **APIs** provide REST endpoints for management and monitoring

## Getting Started

1. **Choose your modules**: Select the modules you need for your use case
2. **Configure plugins**: Add plugins for specialized functionality
3. **Set up configuration**: Create your YAML configuration file
4. **Start the framework**: Launch with your configuration

For detailed setup instructions, see the [development documentation](../development/).
