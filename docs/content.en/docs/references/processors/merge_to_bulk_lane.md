---
title: "merge_to_bulk_lane"
weight: 305
---

# merge_to_bulk_lane

| Category | Scope |
|----------|-------|
| Elasticsearch | pipeline (consumer-form) |

Reads documents from an `input_queue` (disk) **or** an `input_lane` (memory), aggregates them into combined Elasticsearch bulk payloads, and writes the payloads into an in-memory lane. Paired with [`bulk_indexing_lane`](bulk_indexing_lane/) — together they replace the disk-queue bridge (`indexing_merge` + `bulk` queue + `bulk_indexing`) with an all-memory tail: the staging queue is the only disk write. In-flight payloads are lost if the node exits; the lane applies block-on-full backpressure instead of unbounded buffering.

Exactly one of `input_queue` / `input_lane` must be set. The output lane is auto-created (block-on-full, capacity 64) when no `lane:` stanza defines it — define an explicit stanza to tune the memory budget (capacity × `bulk_size_in_mb` is the worst case).

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `input_queue` | string |  | Disk queue to read documents from. |
| `input_lane` | string |  | Lane to read records from — each record's fields are serialized to a document. Exactly one of `input_queue` / `input_lane`. |
| `output_lane` | string | required | Lane receiving the combined bulk payloads. |
| `index_name` | string | required | Target index / data stream name written into the bulk action line. |
| `type_name` | string |  | `_type` for clusters that still require it (defaults from the target cluster's doc type). |
| `write_op_type` | string | "index" | Bulk action (`index` / `create`). Data streams only accept `create`. |
| `key_field` / `key_fields` | string / list |  | Deterministic `_id`: fingerprint of the given field(s) makes replays land on version conflicts instead of duplicates. Without a file source, falls back to hashing the whole document. |
| `bulk_size_in_mb` | int | 10 | Flush the aggregate payload at this size. |
| `bulk_size_in_kb` | int |  | Size trigger in KB (overrides MB). |
| `idle_timeout_in_seconds` | int | 5 | Flush a partial payload after this idle window. |
| `worker_size` | int | 1 | Competing aggregation workers on the input (lane input: workers subscribe in parallel; queue input: workers pop concurrently). |
| `elasticsearch` | string |  | Cluster reference — only used to default `type_name`; the merge never talks to Elasticsearch. |
| `failure_queue` | string | `<input>_failure` | Dead-letter queue for failed payloads. |

## Example

Lane-to-lane aggregation tail (memory-only between staging and the send lane):

```yaml
processor:
  - merge_to_bulk_lane:
      input_lane: ingest-lane-<streamID>
      output_lane: bulk-lane-<streamID>
      index_name: nginx2
      write_op_type: create
      worker_size: 2
      elasticsearch: cls-1
```
