---
title: "otlp_export"
weight: 401
---

# otlp_export

| Category | Scope |
|----------|-------|
| Output transport | pipeline |

The "otlp_export" pipeline processor: the agent side of the OTLP/gRPC transport. It drains a batch of queue messages (the otel JSON envelope produced upstream), renders them as a single OTLP ExportLogsServiceRequest and ships it to one or more collector endpoints (typically the INFINI gateway's OTLP intake on :4317).

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `endpoint` | string |  | OTLP/gRPC endpoint, e.g. `otelcol:4317`. |
| `endpoints` | liststring |  | OTLP/gRPC endpoints; batches are shipped to the first healthy one. |
| `insecure` | bool | true | Skip TLS certificate verification. |
| `timeout` | string | "10s" | Overall request timeout. |
| `headers` | map (string to string) |  | Extra headers sent with each request. |
| `message_field` | string | "messages" | Context key holding the message batch. |
| `max_retries` | int |  | Maximum number of retries. |
| `initial_backoff` | string | "1s" | Initial retry backoff. |
| `max_backoff` | string | "30s" | Upper bound of the retry backoff. |
| `health_cooldown` | string | "10s" | Cooldown before re-checking an unhealthy endpoint. |

## Example

```yaml
processor:
  - otlp_export:
      endpoint: "otelcol:4317"
      endpoints: ["otelcol:4317"]
      insecure: true
      timeout: "30s"
      headers: []
```
