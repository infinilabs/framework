---
title: "csv"
weight: 1001
---

# csv

| Category | Scope |
|----------|-------|
| Parsing | record |

The "csv" pipeline processor: parse a CSV line into named fields (Vector's parse_csv equivalent). Headers come from configuration; missing columns get auto names col_N.

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `field` | string |  | Source field to read from. |
| `headers` | liststring |  | Extra headers sent with each request. |
| `delimiter` | string | "," | Field delimiter. |
| `target_field` | string |  | Destination field to write the result to. |
| `ignore_missing` | bool |  | Do not fail when the source field is missing. |
| `ignore_failure` | bool |  | Do not fail the record when processing errors; the record passes through unchanged. |
| `tag` | string |  | Tag appended to the record when processing fails. |

## Example

```yaml
processor:
  - csv:
      field: "message"
      headers: []
      delimiter: "delimiter"
      target_field: "parsed"
      ignore_missing: true
```
