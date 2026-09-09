---
title: "json"
weight: 1007
---

# json

| Category | Scope |
|----------|-------|
| Parsing | record |

The "json" pipeline processor: decode a JSON string field of the current record into structured attributes.

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `field` | string |  | Source field to read from. |
| `target_field` | string |  | Destination field to write the result to. |
| `overwrite_keys` | bool |  | Overwrite fields that already exist in the record. |
| `ignore_missing` | bool |  | Do not fail when the source field is missing. |
| `ignore_failure` | bool |  | Do not fail the record when processing errors; the record passes through unchanged. |
| `tag` | string |  | Tag appended to the record when processing fails. |

## Example

```yaml
processor:
  - json:
      field: "message"
      target_field: "parsed"
      overwrite_keys: true
      ignore_missing: true
      ignore_failure: true
```
