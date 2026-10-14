# INFINI Framework

## Requirements
- Go 1.21+

## Dependencies

All dependencies are managed via Go modules (`go.mod`). There is no external vendor repository required.

### Internalized Libraries

The following forked/patched libraries live under `lib/` as part of this module:

| Directory | Origin | Notes |
|-----------|--------|-------|
| `lib/seelog` | `github.com/cihub/seelog` | Logging backend; exposed via `replace` directive so existing imports keep working |
| `lib/statsd` | `github.com/quipo/statsd` | StatsD client with custom buffering changes |
| `lib/gomail` | `gopkg.in/gomail.v2` | SMTP client with `NewDialerWithTimeout` extension |
| `lib/tencentcloud` | `github.com/libdns/tencentcloud` | DNS provider with custom signer and types |

### Building

```bash
make build          # production build
make build-dev      # development build (includes -tags dev)
```

No `GOPATH` manipulation or vendor repository checkout is needed. Standard `go build` works directly.
## Pipeline Engine (v2) — Developer Notes

The record-processing engine lives in `core/pipeline` (interfaces, registry,
contracts, codecs, observability) and `modules/pipeline` (the PipeModule
runtime host and `for_each`). Key APIs added by the unified-pipeline work:

### Record codecs (`core/pipeline/codec.go`)

`RecordCodec` converts queue payload bytes to/from `*event.Event` and now
declares the record family it produces via `Namespace()` ("document",
"attachment", "" for generic). Registration accepts runtime callers
(`RegisterRecordCodec` returns an error; `MustRegisterRecordCodec` keeps the
init-time panic), `RegisterRecordCodecOrReplace` upgrades in place
(version-incremented), and `ListRecordCodecs` feeds discovery APIs.
`modules/pipeline` re-exports the API for compatibility. **Domain codec
modules should import only `core/pipeline`** to keep their dependency
surface light (see `infini.sh/coco-codecs` for the reference layout).

### Processor contracts (`core/pipeline/contract.go`)

Processors may implement `ContractProvider` to declare where they may run:

- `Scope` (batch / record / any-zero-value) and `Namespace` are enforced at
  **compile time** by `NewPipelineWithScope(cfg, scope, direct)` — mismatches
  error under `ScopeInfo.Strict`, warn otherwise. Plain `NewPipeline` keeps
  legacy behavior (batch scope, warnings only).
- `Granularity` (record / batch_capable / batch_only): stage-wise execution
  means written order IS execution order; `batch_only` processors must be
  direct members of a record host's `processor:` list (nesting under
  `if/then` is a compile error).
- `CheckContract(ctx, contract)` is the runtime guard for legacy chains.

### Stage-wise execution (`modules/pipeline/for_each.go`)

`for_each` sub-chains execute as alternating segments (record runs / one
batch call) in written order: a batch processor sees the edits of record
processors declared before it; early-segment drops exclude records from all
later segments; clones join the active set and flow through later segments.
A batch-segment failure with `on_failure: tag` tags every live record.

### Execution observers (`core/pipeline/observer.go`)

`ExecutionObserver` receives `StepEvent`s (pipeline, step, processor, scope,
duration, outcome, trace id) from the chain loop — one instrumentation point
covering both batch chains and record sub-chains (scope is derived from the
current record). The empty registry is a zero-allocation fast path. Record
trace ids ride `Metadata.Labels["_trace_id"]` via `SetTraceID`/`TraceID`.
