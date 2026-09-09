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
| `equals` | Exact value match (string, int, float, bool) |
| `contains` | Substring match on strings or string arrays |
| `regexp` | Regular expression match |
| `prefix` / `suffix` | String starts-with / ends-with check |
| `in` | Value membership in a list |
| `range` | Numeric range comparison (`gt`, `gte`, `lt`, `lte`) |
| `exists` | Field existence and non-empty check |
| `length` | Collection/string length equality |
| `network` | IP address network membership (named networks or CIDR) |
| `and` / `or` / `not` | Logical combination and negation |
| `queue_has_lag` / `consumer_has_lag` | Queue or consumer group lag detection |
| `cluster_available` | Elasticsearch cluster availability |

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

The framework ships with a growing set of built-in processors registered via `RegisterProcessorPlugin` / `RegisterDomainProcessor`. The full list can also be discovered at runtime via the `GET /pipeline/processors` API (add `?grouped=1` to group the catalog by category).

### Framework Processors (`modules/pipeline/`)

| Processor | Description |
|-----------|-------------|
| `echo` | Logs a configured message. Useful for debugging and verifying pipeline flow. |
| `dag` | Executes a directed acyclic graph (DAG) of processors, enabling parallel and dependency-based execution within a pipeline. |
| `for_each` | Splits a batch of queue messages into individual records, runs a sub-chain of processors on each record, and re-encodes the batch. Supports pluggable payload codecs, failure strategies (`ignore`/`tag`/`fail`), and drop/clone handling — see the Pipeline Record Processing reference. |

### Queue Processors (`plugins/queue/`)

| Processor | Description |
|-----------|-------------|
| `consumer` | Consumes messages from a queue (selected by name or label selector) and exposes the batch to the pipeline context; supports per-processor consumer config overrides. |
| `queue_output` | Chain-tail companion of `consumer`: appends every record of the processed batch onto a target queue, enabling two-stage pipelines (process on one queue, sink from another). |

### Elasticsearch Processors (`plugins/elastic/`)

| Processor | Description |
|-----------|-------------|
| `bulk_indexing` | Indexes documents into Elasticsearch in bulk for high-throughput ingestion. |
| `json_indexing` | Indexes JSON documents into Elasticsearch. |
| `indexing_merge` | Merges multiple small documents into combined indexing requests before writing to Elasticsearch. |
| `merge_to_bulk` | Merges events into Elasticsearch bulk requests. |

### Output Transports (`plugins/enterprise/otlp/`)

| Processor | Description |
|-----------|-------------|
| `otlp_export` | Ships the processed batch to any OTLP/gRPC collector (e.g. the gateway's intake on `:4317`); unacknowledged batches stay in the local queue for redelivery. |

### Generic Processors (`plugins/`)

| Processor | Description |
|-----------|-------------|
| `http` | Sends HTTP requests to external services. Supports templated URLs, custom headers, and response handling. |
| `smtp` | Sends email notifications via SMTP. |
| `replay` | Replays recorded events for testing or reprocessing. |

### Record Processors (`plugins/enterprise/processors/`)

The record processors operate on the current record inside `for_each` sub-chains (most of them are no-ops outside a record scope). They are registered by domain category; the bare name is the YAML key and `category:name` is an explicit alias.

#### Parsing

| Processor | Description |
|-----------|-------------|
| `dissect` | Fast, regex-free delimiter-based field extraction — the preferred log parsing primitive. |
| `grok` | Regex-based field extraction with the classic `%{PATTERN:name}` syntax and a built-in 350+ definition pattern library (logstash-patterns-core, legacy + ECS v1). |
| `json` | Decodes a JSON string field of the current record into structured attributes. |
| `xml` | XML → map conversion; attributes preserved under `_`-prefixed keys, repeated elements collapse to slices. |
| `csv` | Parses a CSV line into named fields (headers from configuration, missing columns auto-named `col_N`). |
| `kv` | Parses `key=value` pairs out of a string field — the workhorse for access logs, nginx vars and Java GC output. |
| `url` | Splits a URL or `path?query` field into structured parts (scheme, host, port, query params as a map). |
| `urldecode` | URL-decodes string fields (query-string style unescaping). |
| `date` | Parses a timestamp from a field into the record's canonical `Timestamp`. |
| `date_format` | Renders a timestamp field in any layout/timezone (the output direction of `date`). |
| `useragent` | Dependency-free User-Agent classifier (browser, version, OS, device class) with snake_case output. |
| `syslog_decode` | Decodes syslog numeric codes (facility, severity, priority) into names. |
| `web_log` | Preset parsers for the classic web server log formats (Apache/nginx), emitting snake_case access-log fields. |
| `string` | The Graylog string-function toolbox in one processor (substring, split/join/concat, tests, regex replace). |
| `decode_base64_field` | Base64-decodes a single string field (with or without padding). |
| `decompress_gzip_field` | Gunzips a string or bytes field in place. |

#### Transform

| Processor | Description |
|-----------|-------------|
| `mutate` | Field-level mutations on the current record, covering the Logstash mutate filter surface (rename, copy, add, convert, replace, lowercase, uppercase, strip, remove — in a fixed order). |
| `field_standardize` | Normalizes record attribute keys to the canonical INFINI log data model naming (OTel Log Data Model, lowercase snake_case by default). |
| `otel_normalize` | Maps well-known source fields onto the canonical OTel log structure (typed fields + Resource), filling `observed_timestamp` and the derived `severity_number` when absent. |
| `context_enrich` | Promotes collection context — the collecting agent's identity and the envelope's stable resource attributes — into the record's Fields. |
| `extract_array` | Pulls one element (or flattens all elements as indexed fields) out of an array field. |
| `decode_duration` | Converts a Go duration string (`"5s"`, `"1m30s"`) into a numeric duration (milliseconds by default). |

#### Enrichment

| Processor | Description |
|-----------|-------------|
| `geoip` | Enriches the record with geography looked up from a MaxMind mmdb database (GeoLite2/GeoIP2 City, Country, ASN or ISP). |
| `cidr` | Tests an IP field against a list of networks and records the verdict — the natural gate before `geoip` or routing. |
| `registered_domain` | Splits a domain into its registered domain (`www.example.co.uk` → `example.co.uk`) via the Mozilla Public Suffix List. |
| `community_id` | Computes the Community ID flow hash for network events, so flows observed by different tools can be correlated. |
| `fingerprint` | Hashes a set of fields into a stable identity — the deduplication key for repeated events. |
| `enrich_es` | Joins the record against an Elasticsearch/Easysearch index and merges the matched document into it. |
| `pattern_tagger` | Tags each record with the log pattern (template cluster) it belongs to, using LogPilot's merged pattern library. |
| `add_locale` | Stamps each event with the host's local timezone abbreviation or UTC offset. |

#### Routing & Governance

| Processor | Description |
|-----------|-------------|
| `throttle` | Per-key rate limiting over the current records (token bucket); exceeding records are tagged or dropped. |
| `sample` | Probabilistic sampling — keeps each record with probability `ratio` (0.0–1.0), drops the rest. |
| `clone` | Duplicates the current record N times with optional per-clone mutations. |
| `drop_event` | Marks the current record to be dropped from the batch (`for_each` removes its payload). |
| `drop_fields` | Removes fields from the record; with `keep: true` inverts into prune semantics. |
| `redact` | Masks sensitive substrings (phones, national ids, cards, emails, arbitrary regexes) in the configured fields. |

#### Sink & Advanced

| Processor | Description |
|-----------|-------------|
| `pizza_bulk` | Ships the processed batch to a Pizza engine (serve mode) via its Elasticsearch-style `/_bulk` endpoint. |
| `script` | Arbitrary record transforms in ECMAScript (goja) — the escape hatch when declarative processors cannot express the logic. |
