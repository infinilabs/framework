# INFINI Framework

## Pipeline Terminology (glossary)

The data-processing subsystems use a fixed vocabulary. The short version:

| Term | Meaning |
|------|---------|
| **message** | A queue's unit of storage — opaque bytes. Queues never interpret payloads. |
| **batch** | The pipeline's native input: a slice of messages, the *batch scope* all historical processors operate on. |
| **record** | A typed, decoded event inside a record host's sub-chain — the *record scope*; mutable in place, never leaks downstream. |
| **record host** | A splitter processor that turns a batch into records and back: `for_each`, `lane_consumer`, or a custom host. |
| **codec** | Pluggable byte↔record converter (`otel` default, `coco_document`, ...). |
| **namespace** | A record family (`document`, `attachment`) used in compile-time contract checks. |
| **queue** | Durable at-least-once **byte** channel; cross-process safe. |
| **lane** | In-process at-most-once **record** channel (zero serialization, pointer hand-off). |

Full definitions and the queue-vs-lane selection guide live in the docs: [Glossary](docs/content.en/docs/references/glossary.md), [Pipeline Record Processing](docs/content.en/docs/references/pipeline_record.md), [Lane](docs/content.en/docs/references/lane.md).

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