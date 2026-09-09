---
title: "pizza_bulk"
weight: 6001
---

# pizza_bulk

| Category | Scope |
|----------|-------|
| Sink | record |

The "pizza_bulk" output processor: ship the processed batch to a Pizza engine (serve mode) via its Elasticsearch-style /_bulk endpoint. Records are taken from the consumer's message batch (otel envelopes), rendered as NDJSON bulk lines with an index-name template.

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `endpoint` | string |  | OTLP/gRPC endpoint, e.g. `otelcol:4317`. |
| `index` | string | "logs" | Destination index name. |
| `timeout` | string | "10s" | Overall request timeout. |
| `message_field` | string | "messages" | Context key holding the message batch. |

## Example

```yaml
processor:
  - pizza_bulk:
      endpoint: "otelcol:4317"
      index: "index"
      timeout: "30s"
      message_field: "message_field"
```
