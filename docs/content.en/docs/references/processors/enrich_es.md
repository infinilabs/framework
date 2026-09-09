---
title: "enrich_es"
weight: 3003
---

# enrich_es

| Category | Scope |
|----------|-------|
| Enrichment | record |

The "enrich_es" pipeline processor: join the current record against an Elasticsearch/Easysearch index and merge the matched document into the record (port of the gateway's elasticsearch_lookup filter to the log pipeline).

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `elasticsearch` | string | "default" | ID of the registered Elasticsearch/Easysearch cluster. |
| `index_pattern` | string |  | Index name pattern to read from. |
| `match_field` | string |  | Field the pattern match runs against. |
| `es_field` | string | "" | Elasticsearch-side field involved in the enrichment join. |
| `target` | string | "enriched" | Destination prefix or field to write the result to. |
| `cache_ttl` | string |  | TTL of the enrichment lookup cache. |
| `ignore_missing` | bool | true | Do not fail when the source field is missing. |
| `ignore_failure` | bool |  | Do not fail the record when processing errors; the record passes through unchanged. |
| `tag` | string |  | Tag appended to the record when processing fails. |

## Example

```yaml
processor:
  - enrich_es:
      elasticsearch: "default"
      index_pattern: "index_pattern"
      match_field: "match_field"
      es_field: "es_field"
      target: "parsed"
```
