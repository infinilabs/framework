---
title: "lane_consumer"
weight: 211
---

# lane_consumer

| Category | Scope |
|----------|-------|
| Queue | pipeline (batch host) |

Consumes an in-memory lane in competing micro-batches and executes a record sub-chain per batch, stage-wise in written order. Records arrive as live pointers (no codec decode). Surviving records are optionally encoded once into an `output_queue` durable bridge — the memory-to-disk counterpart of `output{lane}`.

Use it to parallelize or re-shape the tail of a lane pipeline: several `lane_consumer` steps (or multiple workers) compete on the same lane, each record is delivered to exactly one of them.

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `lane` | string | required | Lane to consume. Must exist (`lane:` stanza or created by an upstream `output{lane}`). |
| `batch_max` | int | 64 | Records per micro-batch (flush on size). |
| `batch_flush_ms` | int | 100 | Flush an idle partial batch after this window. |
| `on_failure` | string | "tag" | Sub-chain failure strategy: `tag` (default), `dead_letter:<queue>`, or `ignore`. |
| `failure_tag` | string | "_processing_failed" | Tag appended to the record when `on_failure` is `tag`. |
| `output_queue` | string |  | Optional durable bridge: surviving records encode once into this queue. |
| `output_codec` | string |  | Override the codec used for the `output_queue` encode. |
| `processor` | list (processor configs) | required | Record-scope sub-chain executed per batch, stage-wise in written order. |

## Example

Parallel aggregation tail: two workers compete on the raw lane, each aggregates bulk payloads and hands them to the send lane (paired with [`bulk_indexing_lane`](bulk_indexing_lane/)):

```yaml
processor:
  - lane_consumer:
      lane: ingest-lane-<streamID>
      batch_max: 64
      batch_flush_ms: 100
      processor:
        - merge_to_bulk:
            output_queue: { name: bulk-<streamID> }
```
