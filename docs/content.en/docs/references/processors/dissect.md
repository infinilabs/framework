---
title: "dissect"
weight: 1006
---

# dissect

| Category | Scope |
|----------|-------|
| Parsing | record |

The "dissect" pipeline processor: fast, regex-free delimiter-based field extraction, the preferred log parsing primitive of the INFINI data pipeline.

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `pattern` | string |  | Extraction pattern; `%{NAME}` placeholders capture into fields. |
| `field` | string |  | Source field to read from. |
| `target_field` | string |  | Destination field to write the result to. |
| `ignore_missing` | bool |  | Do not fail when the source field is missing. |
| `ignore_failure` | bool |  | Do not fail the record when processing errors; the record passes through unchanged. |
| `overwrite_keys` | bool |  | Overwrite fields that already exist in the record. |
| `tag` | string |  | Tag appended to the record when processing fails. |

## Example

```yaml
processor:
  - for_each:
      processor:
        - dissect:
            field: message
            pattern: "%{client_ip} %{http_method} %{http_path} %{status_code}"
            target_field: http
```
