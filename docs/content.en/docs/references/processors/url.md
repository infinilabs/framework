---
title: "url"
weight: 1011
---

# url

| Category | Scope |
|----------|-------|
| Parsing | record |

The "url" pipeline processor: split a URL or path?query field into structured parts (Vector's parse_url / parse_query_string equivalent). Results land under target (default "url"): scheme, host, path, port, query params as a map.

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `field` | string | "uri" | Source field to read from. |
| `target` | string | "url" | Destination prefix or field to write the result to. |
| `keep_original` | bool |  | Keep the original field after the transformation. |
| `ignore_missing` | bool | true | Do not fail when the source field is missing. |

## Example

```yaml
processor:
  - url:
      field: "message"
      target: "parsed"
      keep_original: true
      ignore_missing: true
```
