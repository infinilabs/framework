---
title: "References"
weight: 20
bookCollapseSection: true
---

# References

Comprehensive reference documentation for the INFINI Framework's core systems and APIs.

## Core Systems

- [Configuration](config/) — YAML configuration management, environment variables, keystore secrets, and config file watching
- [Modules](modules/) — Module lifecycle system for building and registering framework extensions
- [Pipeline & Processors](pipeline/) — Data processing pipelines with conditional logic and custom processor development
- [Processor Reference](processors/) — One reference page per registered processor: configuration parameters, defaults, and examples
- [Task Scheduling](task/) — Interval-based, cron-based, and transient task execution
- [Pipeline Record Processing](pipeline_record/) — The record-processing convention: batch splitting, pluggable payload codecs, failure strategies, and batch-aware processors
- [Queue](queue/) — Pluggable message queue abstraction with disk, memory, Kafka, and Redis backends
- [Key-Value Store](kv/) — Pluggable KV storage with Badger, Elasticsearch, and file-based backends
- [Statistics](stats/) — Metrics collection with counters, gauges, timings, and StatsD integration
- [Conditions](conditions/) — Declarative condition evaluation with logical operators for pipeline control flow

## API & Data

- [API & Web Framework](api_web/) — HTTP API server, web server, routing, middleware, and security configuration
- [Security & Authentication](security/) — Authentication backends (static, native, OAuth, access tokens), unified `/account/login`, and role-based authorization
- [MCP Server](mcp/) — Model Context Protocol server support — expose APIs as AI-callable tools via `api.MCPTool()`
- [ORM](orm/) — Object-Relational Mapping for Elasticsearch with CRUD operations and query building
- [Query URL Parameters](query_url/) — URL-based query parameters for full-text search and structured filters
- [Aggregation Queries](aggs_query/) — Dynamic aggregation construction via URL query parameters

## Operations

- [HTTP Client](http_client/) — HTTP client configuration with proxy, TLS, and connection management
- [Reverse Websocket Channel](websocket_reverse/) — Control-plane-to-agent HTTP proxying over agent-initiated websockets (NAT traversal without inbound ports)
- [Service Management](service/) — Install, start, stop, and uninstall services, including run-as-user setup
- [Keystore](keystore/) — Secure storage for sensitive configuration values
- [Makefile](makefile/) — Build system commands and variables
