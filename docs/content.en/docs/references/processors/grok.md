---
title: "grok"
weight: 8001
---

# grok

| Category | Scope |
|----------|-------|
| Record | record |

The "grok" pipeline processor: regex-based field.

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `field` | string |  | Source field to read from. |
| `patterns` | liststring |  | Extraction patterns tried in order; the first match wins. |
| `pattern_definitions` | map (string to string) |  | custom %{NAME} -> body, overrides built-ins/library |
| `target_field` | string |  | Destination field to write the result to. |
| `overwrite_keys` | bool |  | Overwrite fields that already exist in the record. |
| `ignore_missing` | bool |  | Do not fail when the source field is missing. |
| `ignore_failure` | bool |  | Do not fail the record when processing errors; the record passes through unchanged. |
| `tag` | string |  | Tag appended to the record when processing fails. |

## Example

```yaml
processor:
  - for_each:
      processor:
        - grok:
            field: message
            patterns:
              - "%{IPORHOST:client} %{WORD:verb} %{HTTPDATE:ts}"
            pattern_definitions:
              CUSTOM_CODE: "%{NUMBER:code:int}"
```
