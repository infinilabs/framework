---
title: "References"
weight: 20
bookCollapseSection: true
---

# References

Comprehensive reference documentation for the INFINI Framework's core systems and APIs.

## Core Systems

- [Configuration]({{< relref "config" >}}) — YAML configuration management, environment variables, keystore secrets, and config file watching
- [Modules]({{< relref "modules" >}}) — Module lifecycle system for building and registering framework extensions
- [Pipeline & Processors]({{< relref "pipeline" >}}) — Data processing pipelines with conditional logic and custom processor development
- [Task Scheduling]({{< relref "task" >}}) — Interval-based, cron-based, and transient task execution
- [Queue]({{< relref "queue" >}}) — Pluggable message queue abstraction with disk, memory, Kafka, and Redis backends
- [Key-Value Store]({{< relref "kv" >}}) — Pluggable KV storage with Badger, Elasticsearch, and file-based backends
- [Statistics]({{< relref "stats" >}}) — Metrics collection with counters, gauges, timings, and StatsD integration
- [Conditions]({{< relref "conditions" >}}) — Declarative condition evaluation with logical operators for pipeline control flow

## API & Data

- [API & Web Framework]({{< relref "api_web" >}}) — HTTP API server, web server, routing, middleware, and security configuration
- [ORM]({{< relref "orm" >}}) — Object-Relational Mapping for Elasticsearch with CRUD operations and query building
- [Query URL Parameters]({{< relref "query_url" >}}) — URL-based query parameters for full-text search and structured filters
- [Aggregation Queries]({{< relref "aggs_query" >}}) — Dynamic aggregation construction via URL query parameters

## Operations

- [HTTP Client]({{< relref "http_client" >}}) — HTTP client configuration with proxy, TLS, and connection management
- [Keystore]({{< relref "keystore" >}}) — Secure storage for sensitive configuration values
- [Makefile]({{< relref "makefile" >}}) — Build system commands and variables
