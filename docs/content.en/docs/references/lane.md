---
title: "Lane"
weight: 27
---

# Lane — In-Process Record Channel

A **lane** is the in-process, zero-serialization counterpart of the [queue](../queue/): payloads are `*event.Event` **pointers** — the record *is* the message. A producer emits a record into a lane; a `lane_consumer` host drains it into a record-processing sub-chain. Nothing is encoded or decoded in transit.

```
producer ──Emit(*event.Event)──▶ [lane: bounded, in-flight] ──▶ lane_consumer
                                     │  batch{max, flush_ms}       │
                                     │                             ▼
                              overflow policy              record sub-chain
                              (block/drop/spill)           (same execution
                                                             contract as
                                                             for_each)
```

Use the record-processing model's [conventions](../pipeline_record/) inside the sub-chain — `CurrentRecord`, `MarkDropped`, failure tags all behave identically to `for_each`.

## Declaring Lanes

```yaml
lane:
  - name: mem_processing
    namespace: document        # record-family contract for compile checks
    capacity: 4096             # bounded; in-flight records
    overflow: block            # block | drop | spill:<queue>
  - name: debug_samples
    overflow: drop
```

Configured lanes live in the `lane` section; **dynamic lanes** are created at runtime with `lane.Create(cfg)` / `lane.GetOrCreate(name, cfg)` and carry an `owner` for cascade cleanup (e.g. one lane per compiled stream pipeline, never shared — sharing a lane between consumers is a semantic error). Dynamic lanes default to a small capacity (512) and are lazy: an idle lane costs nothing until first emit. A global `total_budget` caps aggregate in-flight records across dynamic lanes; when the budget is hit each lane applies its own overflow policy.

## Overflow Policies

| Policy | Behavior | When it is correct |
|--------|----------|-------------------|
| `block` (default) | `Emit` waits; backpressure reaches the producer | Whenever the producer can safely slow down (task goroutines, HTTP handlers answering 429). The honest default — data is never risked. |
| `drop` | Discard and count (`drops` metric) | Best-effort telemetry only: tap sampling, debug indexes. |
| `spill:<queue>` | Degrade the record to the disk queue | **Only** when the fast and slow paths run *equivalent* chains — spill means "this record takes the slow version of the same path". |

Two hidden costs of `spill`: a required fast-path step (e.g. redaction) is bypassed for spilled records — a correctness bug if the paths differ; and it can mask backpressure until the spill queue grows unbounded. If the producer cannot block **and** the data must be durable, the honest answer is queue-first (persist, then ACK), not spill.

Spilled records **never flow back into the lane**. The slow path's consumer picks them up on its regular polling cycle, so: a record's path is decided once (`TryEmit` failure is an atomic fork — no double delivery); spilled records survive crashes while in-lane records do not; and completion order across the two paths is not guaranteed — use `block` (lane is FIFO) when ordering matters.

## lane_consumer

```yaml
processor:
  - lane_consumer:
      lane: mem_processing
      batch_max: 200            # flush on size
      batch_flush_ms: 1000      # flush on idle
      on_failure: tag           # tag (default) | dead_letter:<queue> | ignore
      failure_tag: _processing_failed
      output_queue: indexing_documents   # optional durable bridge (encode once)
      processor: [...]          # record sub-chain
```

The execution contract matches `for_each` exactly (stage-wise, order as written, drop/clone/failure-tag semantics identical). Two differences: the entry is zero-decode (records are constructed by the emitting side, not decoded from bytes), and the exit encodes **only** when `output_queue` is configured — `queue.Message.Data = codec.Encode(record)`, wire-compatible with every existing consumer of that codec.

Codec resolution for `output_queue` is three-level: the record's `_codec` hint → the lane's `namespace` primary codec → the default `otel`.

## The Unified `output` Step

Emit into a lane from inside a chain with the [unified output](../pipeline/) step — exactly one target field, enforced at config unpacking:

```yaml
- output:
    lane: mem_processing
    ack: true        # attach a receipt to the context's AckSet
# or:
- output:
    queue: indexing_documents
```

In **batch scope** a queue target pushes raw message bytes; a lane target is a compile error (point the consumer's `codec` + `output_lane` bridging at the lane instead). In **record scope** a queue target encodes the current record; a lane target emits the pointer. `queue_output` / `lane_output` remain as aliases.

## Queue or Lane? — Selection Guide

**The boundary in one sentence:** a successful `Emit` return is the end of the producer's responsibility — lanes are **at-most-once**. A lane flow is safe when at least one of these holds:

1. the upstream can recompute (re-collect, upstream retries, data is disposable), or
2. an uncommitted queue offset covers the whole flow (a crash replays it), or
3. the traffic is best-effort by nature (debugging, sampling).

| Signal | Choose |
|--------|--------|
| Every record must survive crashes | **Queue** — disk persistence, at-least-once redelivery |
| Chain contains non-replayable steps (LLM enrichment, external calls with side effects) | **Queue** (durable path; `for_each` decodes once on each side) |
| Cross-process or cross-machine delivery | **Queue** — lanes are in-process only |
| Light, deterministic pre-processing between an in-process producer and disk | **Lane** (`lane_consumer` + `output_queue` durable bridge; `block` overflow) |
| Webhook-style intake where the caller retries | **Lane** with `block` (backpressure surfaces as 429/slow response) or `drop` |
| Observation taps, sampled debug feeds | **Lane** with `drop` |
| Fan-out after a durable point | **Lane** with `block` — `output{lane:…}` branches after the offset-covered durable write |

### Commit Correctness Across Lanes

`output{lane:…}` is **not** a commit point; a consumer's offset only covers durable writes inside its own sub-chain. Three correct postures, by strength needed:

| Posture | Shape | Guarantee |
|---------|-------|-----------|
| durable-first (default) | `output{queue:…}` inside the sub-chain (the commit covers it); the lane branch only does optional extras | Correct at zero cost — a crash replays from the queue |
| ack receipts | `output{lane:…, ack: true}` settles into the context **AckSet**; the emitting consumer's commit gate waits for settlement (success → `output_queue`, failure → `dead_letter`, explicit drop — all count), undecided or past the consumer's `ack_timeout_ms` (default 30s) → no commit → the disk queue replays | Cross-lane at-least-once: never lost, possibly duplicated (make sinks idempotent — ES `_id` writes are) |
| inline | When "commit only if processed" must be strict, don't decouple: run `for_each` with the durable write in the sub-chain | Strongest and simplest |

## Observability

`GET /pipeline/lanes` returns a live snapshot of every lane: `depth`, `capacity`, in/out counters (`emitted`, `delivered`), `drops`, `spills`, `blocked_waits` (how often `block` overflow actually waited), `owner`, and `closed` — the in-process counterpart of queue-depth observability.
