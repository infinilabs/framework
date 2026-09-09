---
title: "web_log"
weight: 1014
---

# web_log

| Category | Scope |
|----------|-------|
| Parsing | record |

The "web_log" pipeline processor: preset parsers for the classic web server log formats (Vector's parse_apache_log / parse_nginx_log equivalent), emitting snake_case access-log fields.

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `format` | string | "apache_combined" | Preset format selector (e.g. log format). |
| `field` | string |  | Source field to read from. |
| `ignore_missing` | bool |  | Do not fail when the source field is missing. |
| `ignore_failure` | bool |  | Do not fail the record when processing errors; the record passes through unchanged. |
| `tag` | string |  | Tag appended to the record when processing fails. |

## Example

```yaml
processor:
  - web_log:
      format: "format"
      field: "message"
      ignore_missing: true
      ignore_failure: true
      tag: "tag"
```
