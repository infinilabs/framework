---
title: "otel_normalize"
weight: 2006
---

# otel_normalize

| Category | Scope |
|----------|-------|
| Transform | record |

The "otel_normalize" pipeline processor: map well-known source fields of the current record onto the canonical OTel log structure (typed fields + Resource), filling observed_timestamp and the derived severity_number when absent.

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `level_fields` | liststring |  | Fields treated as the log level. |
| `message_fields` | liststring |  | Fields considered as the record's message body, in priority order. |
| `timestamp_fields` | liststring |  | Fields scanned for a parseable timestamp. |
| `trace_fields` | map[string]liststring |  | Per-attribute source fields mapped into the trace context. |
| `resource_fields` | liststring |  | Fields promoted into the record's Resource attributes. |
| `keep_original` | bool |  | Keep the original field after the transformation. |

## Example

```yaml
processor:
  - otel_normalize:
      level_fields: []
      message_fields: []
      timestamp_fields: []
      trace_fields: []
      resource_fields: []
```
