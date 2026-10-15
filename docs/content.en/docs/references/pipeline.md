---
title: "Pipeline and Processor"
weight: 15
---
# Pipeline and Processor

The INFINI Framework includes a pipeline engine for building data processing workflows. A pipeline is an ordered list of processors that execute sequentially against a shared context. Each processor performs a single unit of work — transforming data, calling external services, routing messages, or applying business logic. Pipelines are configured in YAML and can include conditional branching, error handling, and automatic restart behavior.

## Processor Interface

Every processor must implement the `Processor` interface defined in `core/pipeline/processor.go`:

```go
type ProcessorBase interface {
    Name() string
}

type Processor interface {
    ProcessorBase
    Process(s *Context) error
}
```

### Methods

| Method | Description |
|--------|-------------|
| `Name() string` | Returns a unique identifier for the processor, used in configuration, logging, and flow tracking. |
| `Process(s *Context) error` | Executes the processor's logic against the pipeline context. Return `nil` on success or an error to signal failure. |

## Optional Interfaces

Processors may optionally implement additional interfaces for lifecycle management:

```go
type Closer interface {
    Close() error
}

type Releaser interface {
    Release() error
}
```

| Interface | Method | Description |
|-----------|--------|-------------|
| `Closer` | `Close() error` | Called when the pipeline shuts down. Use this to close connections, flush buffers, or clean up resources. |
| `Releaser` | `Release() error` | Called to release resources held by the processor between executions. Useful for freeing temporary allocations while keeping the processor alive. |

If a processor holds external connections (HTTP clients, database handles, file descriptors), implementing `Closer` ensures they are cleaned up gracefully when the pipeline stops.

## Registration

Processors register themselves using `RegisterProcessorPlugin`, which maps a name to a constructor function. Registration typically happens inside a Go `init()` function so that importing the package is sufficient to make the processor available.

```go
// Constructor signature
type ProcessorConstructor func(config *config.Config) (Processor, error)

// Registration function
pipeline.RegisterProcessorPlugin(name string, constructor ProcessorConstructor)
```

### Registration Pattern

```go
func init() {
    pipeline.RegisterProcessorPlugin("echo", NewEchoProcessor)
}
```

Once registered, the processor can be referenced by name in any pipeline's YAML configuration.

## Creating a Custom Processor

Building a custom processor involves three steps: define a config struct, implement the processor, and register it.

### 1. Define a Config Struct

Configuration structs use `config` tags to map YAML keys to Go fields:

```go
type EchoConfig struct {
    Message string `config:"message"`
}
```

### 2. Implement the Processor

The constructor receives a `*config.Config`, unpacks it into the config struct, and returns the processor:

```go
type EchoProcessor struct {
    cfg EchoConfig
}

func NewEchoProcessor(c *config.Config) (pipeline.Processor, error) {
    cfg := EchoConfig{}
    if err := c.Unpack(&cfg); err != nil {
        return nil, fmt.Errorf("failed to unpack the configuration of echo processor: %s", err)
    }
    return &EchoProcessor{cfg: cfg}, nil
}

func (this *EchoProcessor) Name() string {
    return "echo"
}

func (this *EchoProcessor) Process(c *pipeline.Context) error {
    log.Info("message:", this.cfg.Message)
    return nil
}
```

### 3. Register the Processor

```go
func init() {
    pipeline.RegisterProcessorPlugin("echo", NewEchoProcessor)
}
```

## Pipeline Configuration

Pipelines are defined in the application's YAML configuration file under the `pipeline` section. Each pipeline entry has a name, lifecycle flags, and an ordered list of processors.

```yaml
pipeline:
  - name: my_pipeline
    auto_start: true
    keep_running: true
    processor:
      - echo:
          message: "hello world"
      - bulk_indexing:
          elasticsearch: "my-cluster"
```

### Configuration Fields

| Field | Type | Description |
|-------|------|-------------|
| `name` | `string` | Unique name for the pipeline instance. |
| `auto_start` | `bool` | When `true`, the pipeline starts automatically when the application launches. |
| `keep_running` | `bool` | When `true`, the pipeline restarts after completing its processor list, creating a continuous processing loop. |
| `processor` | `list` | Ordered list of processor configurations. Each entry is a map with the processor name as the key and its configuration as the value. |

### Multiple Pipelines

You can define multiple pipelines that run independently:

```yaml
pipeline:
  - name: ingest_pipeline
    auto_start: true
    keep_running: true
    processor:
      - echo:
          message: "ingesting data"

  - name: cleanup_pipeline
    auto_start: true
    keep_running: false
    processor:
      - echo:
          message: "cleanup complete"
```

## Pipeline Context

The `Context` object is passed through every processor in the pipeline and serves as the shared state for a single pipeline execution. Processors read from and write to the context to pass data between stages.

### Key Methods

| Method | Description |
|--------|-------------|
| `ShouldContinue() bool` | Returns `true` if the pipeline should keep executing processors. Check this to respect cancellation or failure signals. |
| `IsCanceled() bool` | Returns `true` if the pipeline run has been explicitly canceled. |
| `AddFlowProcess(name string)` | Records the processor name in the execution history, useful for debugging and tracing the processing flow. |
| `Failed(err error)` | Marks the context as failed with the given error. Subsequent processors can check this to skip work or handle the failure. |

### Using Context in a Processor

```go
func (p *MyProcessor) Process(ctx *pipeline.Context) error {
    if ctx.IsCanceled() {
        return nil
    }

    // Do work...
    result, err := doWork()
    if err != nil {
        ctx.Failed(err)
        return err
    }

    ctx.AddFlowProcess("my_processor")
    return nil
}
```

## The Processors Collection

The `Processors` struct manages an ordered list of processors and executes them sequentially:

```go
type Processors struct {
    SkipCatchError bool
    List           []Processor
}
```

### Functions

| Function / Method | Description |
|-------------------|-------------|
| `NewPipelineList() *Processors` | Creates an empty processor list. |
| `NewPipeline(cfg []*config.Config) (*Processors, error)` | Creates a processor list from a slice of configuration objects, looking up each processor by name in the registry. |
| `AddProcessor(p Processor)` | Appends a processor to the list. |
| `Process(ctx *Context) error` | Executes every processor in order against the given context. |

When `SkipCatchError` is `false` (the default), the pipeline catches errors from individual processors and continues executing the remaining processors. When `true`, errors propagate immediately and halt the pipeline.

## Conditional Processing

Pipelines support `if`/`then`/`else` blocks for conditional execution. The condition is evaluated against the **current record** when the processor runs inside a per-record sub-chain (e.g. `for_each`), so field names refer to the record's own attributes (`file.path`, `log_level`, ...). Outside a record scope the condition is evaluated against the pipeline context, where fields are referenced through the `_ctx.` prefix:

```yaml
processor:
  - if:
      equals:
        _ctx.request.method: "POST"
    then:
      - echo:
          message: "POST request received"
    else:
      - echo:
          message: "non-POST request"
```

The same block inside a `for_each` sub-chain routes each record by its own fields:

```yaml
processor:
  - for_each:
      processor:
        - if:
            contains:
              file.path: "nginx"
          then:
            - mutate:
                add:
                  route: nginx-pipeline
          else:
            - mutate:
                add:
                  route: default-pipeline
```

### Structure

| Field | Description |
|-------|-------------|
| `if` | A condition block (see the operator table below). |
| `then` | A processor (or list of processors) to execute when the condition is `true`. |
| `else` | A processor (or list of processors) to execute when the condition is `false`. Optional. |

### Supported Condition Operators

All operators from the conditions system (`core/conditions/`) are available in `if` blocks — see the Conditions reference for the complete syntax of each:

| Operator | Description |
|----------|-------------|
| [`equals`](conditions/equals/) | Exact value match (string, int, float, bool) |
| [`contains`](conditions/contains/) | Substring match on strings or string arrays |
| [`regexp`](conditions/regexp/) | Regular expression match |
| [`prefix`](conditions/prefix/) / [`suffix`](conditions/suffix/) | String starts-with / ends-with check |
| [`in`](conditions/in/) | Value membership in a list |
| [`range`](conditions/range/) | Numeric range comparison (`gt`, `gte`, `lt`, `lte`) |
| [`exists`](conditions/exists/) | Field existence and non-empty check |
| [`length`](conditions/length/) | Collection/string length equality |
| [`network`](conditions/network/) | IP address network membership (named networks or CIDR) |
| [`and`](conditions/and/) / [`or`](conditions/or/) / [`not`](conditions/not/) | Logical combination and negation |
| [`queue_has_lag`](conditions/queue_has_lag/) / [`consumer_has_lag`](conditions/consumer_has_lag/) | Queue or consumer group lag detection |
| [`cluster_available`](conditions/cluster_available/) | Elasticsearch cluster availability |

### Nested Conditions

Conditions can be nested for complex routing logic:

```yaml
processor:
  - if:
      equals:
        _ctx.request.method: "POST"
    then:
      - if:
          equals:
            _ctx.request.path: "/api/data"
        then:
          - echo:
              message: "POST to /api/data"
        else:
          - echo:
              message: "POST to other path"
    else:
      - echo:
          message: "non-POST request"
```

## Error Handling

Pipeline error handling follows these rules:

1. **Default behavior** — When a processor returns an error, the pipeline logs the error, records it in the context, and continues to the next processor. This prevents a single failing processor from blocking the entire pipeline.

2. **Strict mode** — When `Processors.SkipCatchError` is `true`, the first error returned by any processor stops the pipeline immediately and the error propagates to the caller.

3. **Context failure** — Processors can call `ctx.Failed(err)` to mark the context as failed without returning an error. Downstream processors can check `ctx.ShouldContinue()` to decide whether to skip their work.

4. **Closer cleanup** — When a pipeline shuts down, any processor that implements the `Closer` interface has its `Close()` method called, regardless of whether errors occurred during processing.

### Error Handling Pattern

```go
func (p *MyProcessor) Process(ctx *pipeline.Context) error {
    if !ctx.ShouldContinue() {
        return nil
    }

    err := riskyOperation()
    if err != nil {
        ctx.Failed(err)
        return fmt.Errorf("my_processor failed: %w", err)
    }

    return nil
}
```

## Complete Example

Below is a complete, working example of a custom processor that makes an HTTP health check and records the result in the pipeline context.

### Processor Code

```go
package health

import (
    "fmt"
    "net/http"
    "time"

    log "github.com/cihub/seelog"
    "infini.sh/framework/core/config"
    "infini.sh/framework/core/pipeline"
)

type HealthCheckConfig struct {
    URL     string        `config:"url"`
    Timeout time.Duration `config:"timeout"`
}

type HealthCheckProcessor struct {
    cfg    HealthCheckConfig
    client *http.Client
}

func NewHealthCheckProcessor(c *config.Config) (pipeline.Processor, error) {
    cfg := HealthCheckConfig{
        Timeout: 5 * time.Second,
    }
    if err := c.Unpack(&cfg); err != nil {
        return nil, fmt.Errorf("failed to unpack health_check config: %s", err)
    }
    if cfg.URL == "" {
        return nil, fmt.Errorf("health_check: url is required")
    }
    return &HealthCheckProcessor{
        cfg:    cfg,
        client: &http.Client{Timeout: cfg.Timeout},
    }, nil
}

func (p *HealthCheckProcessor) Name() string {
    return "health_check"
}

func (p *HealthCheckProcessor) Process(ctx *pipeline.Context) error {
    if ctx.IsCanceled() {
        return nil
    }

    resp, err := p.client.Get(p.cfg.URL)
    if err != nil {
        ctx.Failed(err)
        return fmt.Errorf("health check failed for %s: %w", p.cfg.URL, err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        err := fmt.Errorf("unhealthy: %s returned %d", p.cfg.URL, resp.StatusCode)
        ctx.Failed(err)
        return err
    }

    log.Infof("health check passed: %s", p.cfg.URL)
    ctx.AddFlowProcess("health_check")
    return nil
}

// Implement Closer to clean up the HTTP client on shutdown.
func (p *HealthCheckProcessor) Close() error {
    p.client.CloseIdleConnections()
    return nil
}

func init() {
    pipeline.RegisterProcessorPlugin("health_check", NewHealthCheckProcessor)
}
```

### YAML Configuration

```yaml
pipeline:
  - name: monitor_pipeline
    auto_start: true
    keep_running: true
    processor:
      - health_check:
          url: "http://localhost:9200/_cluster/health"
          timeout: "10s"
      - echo:
          message: "cluster is healthy"
```

### Importing the Processor

In your application's main package or plugin registry, import the processor package so its `init()` function runs:

```go
import _ "your_app/plugins/health"
```

## Built-in Processors

The framework ships with a growing set of built-in processors registered via `RegisterProcessorPlugin` / `RegisterDomainProcessor` — framework primitives (`echo`, `dag`, `for_each`), queue bridges (`consumer`, `queue_output`), Elasticsearch indexing (`bulk_indexing`, `json_indexing`, `indexing_merge`, `merge_to_bulk`), output transports (`otlp_export`), general-purpose helpers (`http`, `smtp`, `replay`), and a wide catalog of record processors for parsing, transforming, enriching, routing and governing records inside `for_each` sub-chains (`dissect`, `grok`, `mutate`, `geoip`, `throttle`, `script`, ...).

Every processor has its own reference page — description, configuration parameters with types and defaults, and an example — under the **Processor Reference** section. Record processors operate on the current record inside `for_each` sub-chains (most are no-ops outside a record scope); they are registered by domain category, the bare name is the YAML key, and `category:name` is an explicit alias.

The same catalog is discoverable at runtime via the `GET /pipeline/processors` API (`?grouped=1` groups the catalog by category).

## Configuration Validation

Invalid pipeline configs — unknown processors, missing required parameters, malformed DSL — are rejected **before** they are persisted or distributed, not discovered on the instance after deployment. `ValidatePipelineConfig` (`modules/pipeline/validate.go`) compiles the chain without running it and returns structured findings:

```go
type ConfigError struct {
    Stage     int    `json:"stage"`               // 0-based processor index; -1 = chain/config level
    Processor string `json:"processor,omitempty"` // offending processor key
    Field     string `json:"field,omitempty"`
    Message   string `json:"message"`
}
```

The compile pass covers:

- **DSL shape** — exactly one action per step, well-formed `if`/`then`/`else` blocks, nested sub-chains (`for_each`, `lane_consumer` inner chains compile recursively);
- **Registry existence** — unknown processor names fail with the list of valid names;
- **Parameter legality** — each constructor's own validation runs (missing required fields, bad enum values, out-of-range numbers);
- **Record contracts** — placement mismatches surface as warnings, or as errors when the config opts in with `strict_contract: true`.

Constructor panics are recovered per step and reported as validation errors — a broken plugin can never crash the gate.

### Where the gate is enforced

| Entry point | Behavior on invalid config |
|-------------|---------------------------|
| `POST /pipelines/` (create) | `400` with the findings; nothing persisted |
| `PUT /pipelines/:id` (update) | `400`; the working config and its version history stay untouched |
| `POST /pipeline/tasks/` (start task) | `400` — including disabled pipelines, which would otherwise defer the failure to enable-time |
| `POST /pipelines/:id/versions/:version/rollback` | `400`; a version referencing a processor that was renamed or removed since must not be restored |
| `POST /pipeline/deployments/` (Fleet) | `400`; the referenced pipeline must exist **and** compile (read failures reject too — fail-closed) |
| Deployment reconciler | Refuses to distribute a config that stopped compiling (e.g. a plugin removed after an upgrade) |

Rejection bodies carry both a standard error wrapper (UIs toast `error.reason`) and the structured detail:

```json
{
  "error": { "reason": "invalid pipeline config: http processor requires at least one host (hosts) (+1 more)" },
  "status": 400,
  "errors": [
    { "stage": 1, "processor": "http", "message": "http processor requires at least one host (hosts)" }
  ],
  "warnings": []
}
```

### Pre-flight endpoint

`POST /pipeline/_validate` compiles a submitted config without persisting anything. Always `200`; the `valid` flag carries the verdict:

```bash
curl -X POST /pipeline/_validate -H 'Content-Type: application/json' -d '{
  "name": "my-pipeline",
  "processor": [{ "dissect": { "pattern": "%{a} %{b}" } }]
}'
```

```json
{ "valid": true, "errors": null, "warnings": null }
```

### Constructor contract for processor authors

The gate is only as strong as constructor validation. Two rules, enforced by the `TestEmptyConfigAudit` test:

1. **Return errors, never panic** — a config problem is not a runtime fault; `panic` in a constructor crashes the host that compiles the chain.
2. **Either require your inputs or default them safely** — every processor is constructed with an empty config in CI; one that succeeds must land in the audit's documented allowlist (44 entries, each annotated with why empty-accept is legitimate), otherwise it must fail with a clear required-field error. Variable templates such as `$[[env.HOSTS]]` remain non-empty strings at validation time and do not conflict with required-field checks.

## Pipeline Configuration API

Beyond YAML files, pipelines are managed as first-class resources (`PipelineConfigV2`) over HTTP:

| Method & Path | Description |
|---------------|-------------|
| `POST /pipelines/` | Create a config (validated, versioned). |
| `GET /pipelines/:id` / `PUT /pipelines/:id` / `DELETE /pipelines/:id` | Read / update (validated) / delete (archives the last version first). |
| `POST /pipelines/_search` | Search configs. |
| `POST /pipelines/:id/_start` / `_stop` | Start / stop the pipeline task. |
| `GET /pipelines/:id/versions` | List versions. |
| `GET /pipelines/:id/versions/:version` | Read one version snapshot. |
| `POST /pipelines/:id/versions/:version/rollback` | Restore a version (validated; snapshots the current config first so rollbacks are themselves revertible). |

Every update auto-snapshots the previous config as a `PipelineVersion` — there is no explicit publish step; each edit is a version.

## Pipeline Deployment (Fleet)

`PipelineDeployment` is the control-plane desired state: which pipeline runs on which instances. A reconciler (15s interval) converges desired → reported:

1. Loads the pipeline config, re-validates it compiles, and renders it as a **managed config** row targeted at the selected instances (strategy `all` broadcasts; `singleton` marks the config for instance-side lock resolution).
2. Instances pull the managed config on their next `_sync`, write it to their config directory, and hot-reload the pipeline.
3. Pushes are **converged**: when the rendered content is unchanged, the reconciler skips the version bump — instances don't rewrite files every cycle.

| Method & Path | Description |
|---------------|-------------|
| `POST /pipeline/deployments/` | Create (requires pipeline exists + compiles). |
| `GET /pipeline/deployments/` | List. |
| `GET/PUT/DELETE /pipeline/deployments/:id` | Read / update (enable, placement) / delete. |
| `GET /pipeline/deployments/:id/status` | Desired-vs-reported status. |

### Runtime report and failure visibility

Every managed instance reports its live pipeline states with each config sync (keyed by **pipeline name**). The report persists on the instance record and feeds the deployment status:

```json
{
  "instances": [
    {
      "instance_id": "agent-1",
      "state": "FAILED",
      "reason": "the processor nope does not exist. valid processors: ...",
      "last_seen": "2026-10-15T04:50:54Z"
    }
  ],
  "desired_count": 1,
  "ready_count": 0
}
```

A pipeline whose config fails to compile **on the instance** (e.g. the instance's processor catalog differs from the control plane's — a plugin missing from that binary) is reported as `FAILED` with the compile error as `reason`, instead of silently disappearing from the report. Instances without an ORM backend (lightweight agents) skip the reconciler side but still report runtime state.

## Runtime APIs

| Method & Path | Description |
|---------------|-------------|
| `GET /pipeline/tasks/` | Running pipeline tasks and states. |
| `GET /pipeline/task/:id/stages` | Per-stage metrics: invocations, errors, latency. |
| `GET /pipeline/task/:id/flight` | Sampled execution trace (`?trace_id=`, `?errors=1`, `&limit=`). |
| `POST /pipeline/task/:id/_inject` | Inject a test sample (raw record or batch) into a running pipeline. |
| `POST /pipeline/task/:id/_pause` / `_resume` | Pause / resume a task. |
| `POST /pipeline/dry-run` | Unified dry-run: execute a processor chain against sample input with side-effect capture, field diffs, and scope/contract checks. |
| `GET /pipeline/processors` | Processor catalog (`?grouped=1`). |
| `GET /pipeline/codecs` | Registered record codecs. |
| `GET /pipeline/lanes` | In-process lane channels and their depth. |

`GET /pipeline/cluster/overview` aggregates fleet-wide pipeline health for control-plane dashboards.
