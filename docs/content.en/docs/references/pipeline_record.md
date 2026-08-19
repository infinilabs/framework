---
title: "Pipeline Record Processing"
weight: 26
---

# Pipeline Record Processing

The framework's pipeline natively operates on **batches** of queue messages — opaque byte payloads. The record-processing convention layers a typed, per-record programming model on top: a batch-splitting host processor (the built-in `for_each`, or your own) decodes payloads into `*event.Event` records, runs a sub-chain of record processors over them, and re-encodes. This is the foundation the log-processing ecosystem (dissect, field standardization, enrichment processors) builds on — and it is payload-agnostic: any format with a registered `RecordCodec` participates.

```
consumer ──▶ [batch: []queue.Message]
                │
                ▼
           for_each (splitter host)
                │  decode via RecordCodec (default: otel envelope)
                ├──▶ BatchProcessor.ProcessBatch(records)   ── optional, once per batch
                │
                ├──▶ per record:
                │      ctx.Set(RecordContextKey, rec)
                │      sub-chain processors: mutate / MarkDropped / tag
                │      re-encode via codec
                ▼
           [batch: []queue.Message]  ──▶ downstream (queue_output, otlp_export, ...)
```

## The Convention (core/pipeline/record.go)

Five rules make a processor record-compatible:

1. **Fetch the record** with `CurrentRecord(ctx)` — never decode payloads yourself.
2. **Mutate in place** — `rec.Fields`, `rec.Meta`, `rec.Timestamp` are yours to change.
3. **Drop with `MarkDropped(rec)`** — the host removes the record from the batch; do not null payloads manually.
4. **Tag failures** via `AppendFailureTag(ctx, tag)` when your processor degrades (see [on_failure](#failure-handling)).
5. **Never touch raw bytes** — encoding belongs to the codec; the host re-encodes after your `Process` returns.

```go
func (p *MyProcessor) Process(ctx *pipeline.Context) error {
    rec, ok := pipeline.CurrentRecord(ctx)
    if !ok {
        return nil // not in a record scope — nothing to do
    }
    if v, exists := rec.Fields["message"]; exists {
        rec.Fields["message_len"] = len(v.(string))
    }
    return nil
}
```

Context keys: `RecordContextKey` (the record), `FailureTagsKey` (the record's failure-tag slice, `*[]string`).

## Payload Codecs

Record payloads are converted by pluggable codecs (`modules/pipeline/for_each_codec.go`):

```go
type RecordCodec interface {
    Name() string
    Decode(data []byte) (*event.Event, error)
    Encode(rec *event.Event) ([]byte, error)
}
pipeline.RegisterRecordCodec(myCodec)   // from init(); duplicate names panic
```

The built-in `otel` codec (the default) speaks the otel envelope JSON — byte-compatible with the agent's LogEvent format and the OTLP transport boundary. Register your own for protobuf, msgpack, or domain formats and select it with `codec: <name>`.

Undecodable payloads in a mixed batch **pass through untouched** rather than being dropped — payload transparency for heterogeneous streams.

## for_each Configuration

```yaml
processor:
  - for_each:
      message_field: messages      # ctx key holding []queue.Message (default: messages)
      codec: otel                  # RecordCodec name (default: otel)
      on_failure: ignore           # ignore | tag | fail  — sub-chain error policy
      failure_tag: _processing_failed
      processor:                   # the sub-chain, run per record
        - dissect:
            pattern: "%{log_level} %{message}"
        - field_standardize:
            mode: underscore
```

### Failure Handling

`on_failure` controls what happens when a sub-chain processor returns an error:

| Strategy | Behavior |
|----------|----------|
| `ignore` (default) | Warn, keep the (possibly partially mutated) record, and skip the rest of the sub-chain for that record — the historical behavior |
| `tag` | Append `failure_tag` to the record's failure tags and keep processing the rest of the sub-chain; downstream processors read them via `FailureTagsKey`/`CurrentFailureTags`, and the accumulated tags are persisted into the record's `Fields["tags"]` before re-encoding so later pipeline stages can route on them |
| `fail` | Abort the batch: `Process` returns the error, the consumer leaves the offset uncommitted, the queue redelivers (at-least-once) |

## Batch-Aware Processors

Processors that work better on a whole batch (sampling, rate limiting, aggregation, batched lookups) implement the optional interface:

```go
type BatchProcessor interface {
    Processor
    ProcessBatch(ctx *Context, records []*event.Event) error
}
```

The host detects implementers and calls `ProcessBatch` **once per batch** after decoding — **before** any per-record processor, regardless of the batch processor's position in the sub-chain; plain `Process` runs per record as before. Two rules for implementers:

- Drop records with `MarkDropped` — the host honors the marker in the encode pass.
- **Do not reslice/compact the `records` slice** — the caller owns the backing array; mutate through the pointers only.

Implementing `ProcessBatch` does not relieve you of `Process`: non-batch-aware hosts still call the per-record path.

## Nesting and Composition

A sub-chain may itself contain another splitter bound to a different message field — per-record state is scoped by the splitter: it saves and restores `RecordContextKey` (and `FailureTagsKey`) around the batch, so the last record never leaks to processors downstream of `for_each` and nested splitters restore the outer record scope. Conditional processors (`if`, `switch`, dag) compose inside sub-chains for per-record routing.

## Writing a Splitter Host

`for_each` is the reference implementation; a custom host follows the same skeleton:

1. Read the batch from the configured context key.
2. Save the previous `RecordContextKey`/`FailureTagsKey` values and restore them on return (scope isolation — no record leaks downstream).
3. Decode pass → `[]*event.Event` + index mapping (skip empties; pass undecodables through).
4. Batch pass → `ProcessBatch` on implementers (before the per-record pass).
5. Record pass → set `RecordContextKey` (+ `FailureTagsKey` when tagging), run plain processors, apply `on_failure`.
6. Encode pass → honor `IsDropped`, persist failure tags (`Fields["tags"]`), re-encode, write back to the batch.

## Compatibility Notes

- Sub-processors written against the original for_each (pre-convention) work unchanged: per-record `Process`, drop markers, and `ignore`-on-error semantics (error skips the rest of the sub-chain for that record) are preserved exactly.
- The `otel` codec's wire format is unchanged: the envelope decode→encode pass is byte-stable for agent metadata and tolerates bare JSON map payloads; the codec layer is an internal refactor of the previous direct decode calls.
