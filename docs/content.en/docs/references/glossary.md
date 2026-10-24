---
title: "Glossary"
weight: 28
---

# Glossary

Canonical terminology for the pipeline, record-processing, and lane subsystems. When a term appears in configuration, code, or UI, this is what it means.

## Messages, Batches, Records

| Term | Meaning |
|------|---------|
| **message** | A queue's unit of storage: an opaque byte payload (`queue.Message`). Queues do not interpret messages. |
| **batch** | The pipeline's native input: a slice of messages delivered to a chain. All pipeline processors historically operate here (the *batch scope*). |
| **record** | A typed, decoded event (`*event.Event`) produced inside a record host. Fields, metadata, and timestamps are mutable in place. Records exist only inside a host's scope — they never leak to the outer batch chain. |
| **record host** (splitter host) | A processor that splits a batch into records, runs a sub-chain over them, and re-encodes: the built-in `for_each`, `lane_consumer`, or a custom host following the same skeleton. |
| **codec** (`RecordCodec`) | Pluggable encoder/decoder between message bytes and records (`otel` default, `coco_document`, `coco_attachment`, ...). Registered by name; selected with `codec:` on `for_each`, resolved three-level for output queues. |
| **namespace** | A record family identifier (`document`, `attachment`, ...) declared by codecs and lane configs; the compile-time currency of `RecordContract.Namespace`. |

## Scopes and Contracts

| Term | Meaning |
|------|---------|
| **batch scope** | Execution context at the chain root: processors see raw messages and pipeline parameters, not records. |
| **record scope** | Execution context inside a record host's sub-chain: `CurrentRecord` is bound, record field paths (`document.content`) are addressable in conditions. |
| **RecordContract** | A processor's compile-time declaration: where it may run (`Scope`), which record family it needs (`Namespace`), which paths it asserts (`RequiredFields`), and its `Granularity`. Unconstrained processors pass every placement. |
| **granularity** | Which execution channels a processor supports: `record` (per-record path only), `batch_capable` (both, batch preferred), `batch_only` (cross-record aggregation; must sit directly in a record host's sub-chain). |
| **strict_contract** | Pipeline config flag: placement violations become compile errors instead of warnings. The `POST /pipeline/_validate` gate reports them either way. |
| **failure tag** | A record-level error marker appended by `on_failure: tag`; accumulated tags persist into the record's `tags` field before re-encoding so downstream stages can route on them. |

## Channels

| Term | Meaning |
|------|---------|
| **queue** | Durable, at-least-once byte channel between producers and consumers (disk, memory, Kafka, Redis backends). Cross-process safe. |
| **lane** | In-process, zero-serialization record channel: payloads are `*event.Event` pointers. Bounded, at-most-once past a successful `Emit`. See [Lane](../lane/). |
| **overflow policy** | A full lane's behavior: `block` (backpressure, default), `drop` (best-effort), `spill:<queue>` (degrade to disk — only for equivalent fast/slow paths). |
| **durable bridge** (`output_queue`) | A `lane_consumer`'s optional exit: surviving records encode once into a disk queue, making the lane flow's outcome persistent and wire-compatible with existing consumers. |
| **AckSet** | Context-borne receipt set for `output{lane:…, ack: true}` steps; the emitting consumer's commit gate waits for settlement before committing offsets (cross-lane at-least-once). |

## Control Plane and Debugging

| Term | Meaning |
|------|---------|
| **processor catalog** | Discovery view of all registered processors (scope, contract, config schema) — `GET /pipeline/processors`. Drives canvas UIs and schema forms. |
| **dry-run** | Server-side chain execution against a sample with side effects replaced by captures: step snapshots, field diffs, per-step timings. `POST /pipeline/dry-run`. |
| **placement** | Compile-time check that a processor's scope/granularity matches its position (e.g. a `batch_only` processor must sit directly in a record host's sub-chain). |
| **dynamic lane** | A lane created at runtime (`lane.Create` / `GetOrCreate`) with an `owner` for cascade cleanup — e.g. one per compiled stream pipeline, never shared. |
| **canvas_layout** | Product-owned pipeline config field holding canvas node coordinates (`map[string]{x,y}`, content-fingerprint keys). Pure presentation state; execution never reads it. |
