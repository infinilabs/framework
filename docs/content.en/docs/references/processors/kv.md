---
title: "kv"
weight: 1008
---

# kv

| Category | Scope |
|----------|-------|
| Parsing | record |

The "kv" pipeline processor: parse key=value pairs out of a string field — the workhorse for access logs, nginx vars and Java GC output.

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `field` | string |  | Source field to read from. |
| `field_split` | string | " " | Separator between key/value pairs. |
| `value_split` | string | "=" | Separator between a key and its value. |
| `target_field` | string |  | Destination field to write the result to. |
| `trim_key` | string |  | Trim these characters from parsed keys. |
| `trim_value` | string |  | Trim these characters from parsed values. |
| `overwrite_keys` | bool |  | Overwrite fields that already exist in the record. |
| `ignore_missing` | bool |  | Do not fail when the source field is missing. |
| `ignore_failure` | bool |  | Do not fail the record when processing errors; the record passes through unchanged. |
| `tag` | string |  | Tag appended to the record when processing fails. |

## Example

```yaml
processor:
  - kv:
      field: "message"
      field_split: "field_split"
      value_split: "value_split"
      target_field: "parsed"
      trim_key: "trim_key"
```
