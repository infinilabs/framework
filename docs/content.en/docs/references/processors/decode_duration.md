---
title: "decode_duration"
weight: 2002
---

# decode_duration

| Category | Scope |
|----------|-------|
| Transform | record |

The "decode_duration" pipeline processor: convert a Go duration string like "5s" or "1m30s" into a numeric duration (milliseconds by default).

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `field` | string |  | Source field to read from. |
| `format` | string |  | Preset format selector (e.g. log format). |

## Example

```yaml
processor:
  - decode_duration:
      field: "message"
      format: "format"
```
