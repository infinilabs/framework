---
title: "indexing_merge"
weight: 302
---

# indexing_merge

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
| `index_name` | string |  | Destination index name; supports templating. |
| `type_name` | string |  | Elasticsearch mapping type of the documents. |
| `write_op_type` | string |  | create, index, update |
| `key_field` | string |  | the field name used as document's primary key aka `_id |
| `key_fields` | liststring |  | Fields hashed or grouped into the rate-limit key. |
| `elasticsearch` | string |  | ID of the registered Elasticsearch/Easysearch cluster. |
| `input_queue` | string |  | Queue the batches are pulled from. |
| `name` | string |  | Identifier of the resource this processor binds to. |
| `label` | map |  | Static labels attached to the documents handled by this processor. |
| `failure_queue` | string |  | Queue that receives failed events. |
| `invalid_queue` | string |  | Queue that receives malformed events. |

## Example

```yaml
processor:
  - indexing_merge:
      worker_size: 10
      idle_timeout_in_seconds: 10
      bulk_size_in_kb: 10
      bulk_size_in_mb: 10
      index_name: "index_name"
```
