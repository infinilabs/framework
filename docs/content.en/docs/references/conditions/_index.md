---
title: "Conditions"
weight: 45
---
# Conditions

The INFINI Framework provides a conditions system for evaluating events against configurable rules. Conditions are used throughout the framework — most notably in pipeline `if`/`then`/`else` branching — to make runtime decisions based on field values, patterns, numeric ranges, network membership, and logical combinations.

Each operator has its own reference page under this section, kept in a uniform format: what it tests, its parameters, and examples.

## Condition Interface

Every condition implements the `Condition` interface:

```go
type Condition interface {
    Check(event ValuesMap) bool
    String() string
}
```

| Method | Description |
|--------|-------------|
| `Check(event ValuesMap) bool` | Evaluates the condition against an event. Returns `true` if the event matches. |
| `String() string` | Returns a human-readable representation of the condition for logging and debugging. |

Events are read through the `ValuesMap` interface, which provides dot-notation field access:

```go
type ValuesMap interface {
    GetValue(string) (interface{}, error)
}
```

## Evaluation Scope

The condition's field lookups resolve against different sources depending on where the `if` block runs:

- **Per-record sub-chains** (e.g. inside `for_each`): conditions evaluate against the **current record**, and field names refer to the record's own attributes (`file.path`, `log_level`, ...) — dot notation walks nested fields.
- **Pipeline level** (no record bound): conditions evaluate against the **pipeline context**, where fields are referenced through the `_ctx.` prefix (e.g. `_ctx.request.method`).

If a field does not exist, `GetValue` returns an error and the condition typically evaluates to `false`.

## Config Struct

Conditions are declared in YAML configuration and deserialized into the `Config` struct (`core/conditions/`):

```go
type Config struct {
    Equals           *Fields                `config:"equals"`
    Contains         *Fields                `config:"contains"`
    Prefix           map[string]interface{} `config:"prefix"`
    Suffix           map[string]interface{} `config:"suffix"`
    Regexp           *Fields                `config:"regexp"`
    Range            *Fields                `config:"range"`
    Network          map[string]interface{} `config:"network"`
    Exists           []string               `config:"exists"`
    IN               map[string]interface{} `config:"in"`
    LengthEquals     *Fields                `config:"length"`
    OR               []Config               `config:"or"`
    AND              []Config               `config:"and"`
    NOT              *Config                `config:"not"`
    QueueHasLag      []string               `config:"queue_has_lag"`
    ConsumerHasLag   *Fields                `config:"consumer_has_lag"`
    ClusterAvailable []string               `config:"cluster_available"`
}
```

A `Config` must contain exactly **one** top-level operator. To combine multiple operators, use the logical operators [`and`](and/), [`or`](or/), or [`not`](not/).

## Operator Reference

| Operator | Kind | Description |
|----------|------|-------------|
| [`equals`](equals/) | Value | Exact value match (string, int, float, bool) |
| [`contains`](contains/) | Value | Substring match on strings or string arrays |
| [`regexp`](regexp/) | Value | Regular expression match |
| [`prefix`](prefix/) | Value | String starts-with check |
| [`suffix`](suffix/) | Value | String ends-with check |
| [`in`](in/) | Value | Value membership in a list |
| [`range`](range/) | Value | Numeric range comparison (`gt`, `gte`, `lt`, `lte`) |
| [`exists`](exists/) | Value | Field existence and non-empty check |
| [`length`](length/) | Value | Collection/string length equality |
| [`network`](network/) | Value | IP address network membership |
| [`and`](and/) | Logical | Logical AND (all must match) |
| [`or`](or/) | Logical | Logical OR (any must match) |
| [`not`](not/) | Logical | Logical negation |
| [`queue_has_lag`](queue_has_lag/) | Domain | Message queue lag detection |
| [`consumer_has_lag`](consumer_has_lag/) | Domain | Consumer group lag detection |
| [`cluster_available`](cluster_available/) | Domain | Elasticsearch cluster availability |

## Using Conditions in Pipelines

Conditions power the `if`/`then`/`else` branching in pipeline processor definitions. The `if` block takes a single condition configuration. When it evaluates to `true`, the `then` processors execute; otherwise, the `else` processors run (if provided).

### Basic Branching

```yaml
pipeline:
  - name: my_pipeline
    auto_start: true
    keep_running: true
    processor:
      - if:
          equals:
            _ctx.request.method: "POST"
        then:
          - echo:
              message: "POST request received"
        else:
          - echo:
              message: "Non-POST request"
```

### Per-Record Routing

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

### Complex Conditions

```yaml
pipeline:
  - name: api_filter
    auto_start: true
    keep_running: true
    processor:
      - if:
          and:
            - equals:
                _ctx.request.method: "GET"
            - not:
                contains:
                  _ctx.request.uri: "/health"
            - exists:
                - _ctx.request.header.Authorization
          then:
            - echo:
                message: "Authenticated GET request (non-health)"
```

## Using Conditions Programmatically

You can create and evaluate conditions directly from Go code:

```go
import "infini.sh/framework/core/conditions"

cfg := &conditions.Config{
    Equals: &conditions.Fields{},
}
// Typically populated by config deserialization, or manually:
cfg.Equals = conditions.MustNewFields(map[string]interface{}{
    "type": "process",
    "proc.pid": 305,
})

cond, err := conditions.NewCondition(cfg)
if err != nil {
    log.Fatal(err)
}

// Check against an event that implements ValuesMap
if cond.Check(event) {
    fmt.Println("Condition matched")
}
```

### Building Compound Conditions

```go
cfg := &conditions.Config{
    AND: []conditions.Config{
        {
            Equals: conditions.MustNewFields(map[string]interface{}{
                "status": "active",
            }),
        },
        {
            Range: conditions.MustNewFields(map[string]interface{}{
                "age.gte": 18,
            }),
        },
    },
}

cond, err := conditions.NewCondition(cfg)
```

### Using the Context Helper

The `Context` type aggregates multiple `ValuesMap` sources, so a condition can read fields from several data providers. It also supports variable templates with `$[[variable]]` syntax:

```go
ctx := &conditions.Context{}
ctx.AddContext(primaryData)
ctx.AddContext(fallbackData)

if cond.Check(ctx) {
    // Fields are looked up across all added contexts in order
}
```
