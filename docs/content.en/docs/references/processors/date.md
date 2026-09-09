---
title: "date"
weight: 1002
---

# date

| Category | Scope |
|----------|-------|
| Parsing | record |

The "date" pipeline processor: parse a timestamp from a field of the current record into the record's canonical Timestamp (OTel: the time the event occurred, source clock).

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `field` | string |  | Source field to read from. |
| `formats` | liststring |  | Timestamp layouts tried in order. |
| `timezone` | string |  | Timezone assumed when parsing or rendering. |
| `ignore_missing` | bool |  | Do not fail when the source field is missing. |
| `ignore_failure` | bool |  | Do not fail the record when processing errors; the record passes through unchanged. |
| `tag` | string |  | Tag appended to the record when processing fails. |

## Example

```yaml
processor:
  - for_each:
      processor:
        - date:
            field: timestamp
            layouts:
              - "yyyy-MM-dd HH:mm:ss"
              - "ISO8601"
            timezone: Asia/Shanghai
```
