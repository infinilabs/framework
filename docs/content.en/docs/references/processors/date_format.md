---
title: "date_format"
weight: 1003
---

# date_format

| Category | Scope |
|----------|-------|
| Parsing | record |

The "date_format" pipeline processor: the output direction Graylog's format_date covers — read a timestamp field (already normalized by `date`, or RFC3339-ish) and render it in any layout/timezone.

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `source` | string |  | Source field to read from. |
| `target_field` | string | "formatted_time" | Destination field to write the result to. |
| `to_timezone` | string |  | Timezone the timestamp is rendered in. |
| `to_format` | string |  | Output layout of the rendered timestamp. |

## Example

```yaml
processor:
  - date_format:
      source: "message"
      target_field: "parsed"
      to_timezone: "to_timezone"
      to_format: "to_format"
```
