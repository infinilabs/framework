---
title: "json_indexing"
weight: 303
---

# json_indexing

| Category | Scope |
|----------|-------|
| Elasticsearch | pipeline |

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `worker_size` | int | 1 | Number of parallel workers. |
| `idle_timeout_in_seconds` | int | 5 | Seconds of inactivity before a worker parks. |
| `bulk_size_in_kb` | int |  | Maximum bulk request size in kilobytes. |
| `bulk_size_in_mb` | int | 10 | Maximum bulk request size in megabytes. |
| `index_prefix` | string |  | Prefix prepended to generated index names. |
| `index_name` | string |  | Destination index name; supports templating. |
| `type_name` | string |  | Elasticsearch mapping type of the documents. |
| `elasticsearch` | string |  | ID of the registered Elasticsearch/Easysearch cluster. |
| `input_queue` | string |  | Queue the batches are pulled from. |
| `failure_queue` | string |  | Queue that receives failed events. |
| `invalid_queue` | string |  | Queue that receives malformed events. |
| `check_available` | bool |  | Verify the target cluster is available before sending. |

## Example

```yaml
processor:
  - json_indexing:
      worker_size: 10
      idle_timeout_in_seconds: 10
      bulk_size_in_kb: 10
      bulk_size_in_mb: 10
      index_prefix: "index_prefix"
```
