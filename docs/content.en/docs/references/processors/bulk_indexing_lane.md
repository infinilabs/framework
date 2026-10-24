---
title: "bulk_indexing_lane"
weight: 306
---

# bulk_indexing_lane

| Category | Scope |
|----------|-------|
| Elasticsearch | pipeline (consumer-form) |

Consumes combined bulk payloads from an in-memory lane and ships them to Elasticsearch. Paired with [`merge_to_bulk_lane`](merge_to_bulk_lane/) — the send half of the all-memory indexing tail. One batch's payloads are sent concurrently across `worker_size` workers; the batch function waits for every send before returning, keeping receipt settlement exact.

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `lane` | string | required | Lane carrying combined bulk payloads (from `merge_to_bulk_lane.output_lane`). |
| `elasticsearch` | string | required | Cluster reference (id resolved Gateway-side; no credentials inlined). |
| `worker_size` | int |  | Send one batch's payloads concurrently across workers. |
| `bulk` | object |  | Bulk processor tuning: `batch_size_in_mb`, `batch_size_in_docs`, `retry_delay_in_seconds`, `max_reject_retry_times`, `request_timeout_in_second`, dead-letter queues. |
| `failure_queue` | string |  | Dead-letter queue for payloads that exhausted retries. |
| `max_retries` | int |  | Send retry attempts before dead-lettering. |
| `retry_delay_in_ms` | int |  | Delay between send retries. |

## Example

```yaml
processor:
  - bulk_indexing_lane:
      lane: bulk-lane-<streamID>
      elasticsearch: cls-1
      worker_size: 4
```
