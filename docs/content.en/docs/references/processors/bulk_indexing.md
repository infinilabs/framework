---
title: "bulk_indexing"
weight: 301
---

# bulk_indexing

| Category | Scope |
|----------|-------|
| Elasticsearch | pipeline |

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `num_of_slices` | int | 1 | Number of slices the work is split into. |
| `slices` | listint |  | Number of parallel slices. |
| `document_level_slicing` | bool |  | Slice batches at document level instead of offset level. |
| `idle_timeout_in_seconds` | int | 5 | Seconds of inactivity before a worker parks. |
| `max_connection_per_node` | int | 1 | Connection pool size per host. |
| `queues` | map |  | Queue names to consume from. |
| `queue_selector` | queue.QueueSelector |  | Label selector that picks queues by their tags (ALL pairs must match). |
| `consumer` | queue.ConsumerConfig |  | Nested consumer settings (group, labels, auto_reset_offset, ...) applied on top of the shared registry config. |
| `max_worker_size` | int | 10 | Upper bound for the worker count. |
| `double_check_offset_before_bulk` | bool |  | Re-verify the offset before committing it after a bulk. |
| `detect_active_queue` | bool | true | Only pick up queues that have pending messages. |
| `verbose_bulk_result` | bool | false | Include per-item results in the logs. |
| `detect_interval` | int | 5000 | How often to probe for new queues. |
| `valid_request` | bool | false | Request validator; matching requests are considered valid. |
| `skip_empty_queue` | bool | true | Advance to the next queue or segment when this one is empty. |
| `skip_info_missing` | bool | false | Skip events whose required metadata is missing instead of failing them. |
| `log_bulk_error` | bool | true | Log the bulk response body when items fail. |
| `bulk` | elastic.BulkProcessorConfig |  | Nested bulk-processor settings of the Elasticsearch client (batch size, flush, retry policy). |
| `elasticsearch` | string |  | ID of the registered Elasticsearch/Easysearch cluster. |
| `elasticsearch_config` | *elastic.ElasticsearchConfig |  | Inline Elasticsearch connection settings, used instead of a registered cluster ID. |
| `waiting_after` | liststring |  | Pause after finishing a sweep before the next one. |
| `retry_delay_interval` | int | 5000 | Delay between retries. |

## Example

```yaml
processor:
  - bulk_indexing:
      num_of_slices: 10
      slices: []
      document_level_slicing: true
      idle_timeout_in_seconds: 10
      max_connection_per_node: 10
```
