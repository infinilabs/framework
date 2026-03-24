---
title: "Conditions"
weight: 45
---
# Conditions

The INFINI Framework provides a powerful conditions system for evaluating events against configurable rules. Conditions are used throughout the framework — most notably in pipeline `if`/`then`/`else` branching — to make runtime decisions based on field values, patterns, numeric ranges, network membership, and logical combinations.

The conditions package is located at `core/conditions/`.

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

Fields are referenced using dot notation (e.g., `_ctx.request.method`, `http.response.status_code`). If a field does not exist, `GetValue` returns an error and the condition typically evaluates to `false`.

## Config Struct

Conditions are declared in YAML configuration and deserialized into the `Config` struct:

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

A `Config` is converted into a live `Condition` using the factory functions:

```go
func NewCondition(config *Config) (Condition, error)
func NewConditionList(config []Config) ([]Condition, error)
```

Each `Config` must contain exactly **one** top-level operator. To combine multiple operators, use the logical operators `and`, `or`, or `not`.

## Value Operators

### equals

Tests whether a field value is exactly equal to an expected value. Supports `string`, `int`, `float`, and `bool` types. When multiple fields are specified, **all** must match (implicit AND).

```yaml
equals:
  _ctx.request.method: "GET"
```

```yaml
# Multiple fields — all must match
equals:
  type: "process"
  proc.pid: 305
```

### contains

Tests whether a string field contains a given substring. Also works on arrays of strings — returns `true` if any element contains the substring.

```yaml
contains:
  _ctx.request.uri: "/api"
```

### regexp

Tests whether a string field matches a regular expression pattern. Also supports matching against arrays of strings.

```yaml
regexp:
  source: "apache2/error.*"
```

```yaml
regexp:
  message: "[Ee]rror|[Ff]ailed"
```

### prefix

Tests whether a string field starts with a given prefix. Accepts exactly one field.

```yaml
prefix:
  hostname: "prod-"
```

### suffix

Tests whether a string field ends with a given suffix. Accepts exactly one field.

```yaml
suffix:
  filename: ".log"
```

### in

Tests whether a field value is contained in a list of allowed values. Supports both string and integer values.

```yaml
in:
  _ctx.response.status_code: [200, 201, 204]
```

```yaml
in:
  env: ["production", "staging"]
```

### range

Tests whether a numeric field falls within a specified range. Supports the following comparison operators:

| Operator | Meaning |
|----------|---------|
| `gte` | Greater than or equal to (>=) |
| `gt` | Greater than (>) |
| `lte` | Less than or equal to (<=) |
| `lt` | Less than (<) |

Range operators are appended to the field name with a dot separator:

```yaml
# 200 <= status_code < 300
range:
  _ctx.response.status_code:
    gte: 200
    lt: 300
```

```yaml
# CPU usage above 90%
range:
  proc.cpu.total_p:
    gt: 0.9
```

Supports `int`, `uint`, and `float` numeric types.

### exists

Tests whether one or more fields exist and are non-empty. Accepts a list of field names. All fields must exist for the condition to match.

```yaml
exists:
  - username
  - email
  - session_id
```

### length

Tests whether the length of a field's value equals an expected integer. Works with slices, arrays, strings, maps, and channels.

```yaml
length:
  tags: 3
```

### network

Tests whether an IP address field belongs to a specific network. Accepts named network identifiers or CIDR notation.

**Named networks:**

| Name | Description |
|------|-------------|
| `loopback` | Loopback addresses (e.g., `127.0.0.1`, `::1`) |
| `private` | RFC 1918 (IPv4) and RFC 4193 (IPv6) private addresses |
| `public` | Any address that is not local or private |
| `global_unicast` | Global unicast addresses |
| `unicast` | Alias for `global_unicast` |
| `link_local_unicast` | Link-local unicast addresses |
| `multicast` | Multicast addresses |
| `link_local_multicast` | Link-local multicast addresses |
| `interface_local_multicast` | Interface-local multicast addresses |
| `unspecified` | The unspecified address (`0.0.0.0` or `::`) |

```yaml
# Named network
network:
  client_ip: private
```

```yaml
# CIDR notation
network:
  source.ip: "192.168.1.0/24"
```

Multiple networks can be specified as a list — the field matches if it belongs to **any** of them:

```yaml
network:
  client_ip: ["private", "loopback"]
```

## Logical Operators

Logical operators combine or negate conditions to build complex expressions.

### and

Evaluates to `true` only when **all** inner conditions are true. Uses short-circuit evaluation — stops checking on the first `false`.

```yaml
and:
  - equals:
      _ctx.request.method: "POST"
  - contains:
      _ctx.request.uri: "/api"
```

### or

Evaluates to `true` when **any** inner condition is true. Uses short-circuit evaluation — stops checking on the first `true`.

```yaml
or:
  - equals:
      _ctx.response.status_code: 401
  - equals:
      _ctx.response.status_code: 403
```

### not

Negates a single inner condition. Evaluates to `true` when the inner condition is `false`.

```yaml
not:
  contains:
    _ctx.request.uri: "/health"
```

### Nesting Logical Operators

Logical operators can be nested to any depth:

```yaml
and:
  - equals:
      _ctx.request.method: "GET"
  - not:
      contains:
        _ctx.request.uri: "/internal"
  - or:
      - prefix:
          _ctx.request.uri: "/api/v1"
      - prefix:
          _ctx.request.uri: "/api/v2"
```

## Domain-Specific Conditions

### queue_has_lag

Tests whether a message queue has unconsumed messages. Accepts a list of queue specifiers. An optional `> max_depth` threshold can be appended:

```yaml
queue_has_lag:
  - "my_queue"
  - "my_queue > 1000"
```

### consumer_has_lag

Tests whether a consumer group has fallen behind the producer on a queue:

```yaml
consumer_has_lag:
  queue: "my_queue"
  group: "consumer_group"
  name: "consumer_1"
```

### cluster_available

Tests whether one or more Elasticsearch clusters are available:

```yaml
cluster_available:
  - "primary_cluster"
  - "backup_cluster"
```

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

### Range-Based Routing

```yaml
pipeline:
  - name: error_handler
    auto_start: true
    keep_running: true
    processor:
      - if:
          range:
            _ctx.response.status_code:
              gte: 400
              lt: 500
        then:
          - echo:
              message: "Client error (4xx)"
      - if:
          range:
            _ctx.response.status_code:
              gte: 500
        then:
          - echo:
              message: "Server error (5xx)"
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

### Nested If/Then/Else

```yaml
pipeline:
  - name: nested_routing
    auto_start: true
    keep_running: true
    processor:
      - if:
          equals:
            _ctx.request.method: "POST"
        then:
          - if:
              prefix:
                _ctx.request.uri: "/api/"
            then:
              - echo:
                  message: "POST to API"
            else:
              - echo:
                  message: "POST to non-API"
        else:
          - echo:
              message: "Non-POST request"
```

## Using Conditions Programmatically

You can create and evaluate conditions directly from Go code:

### Creating a Condition from Config

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
import "infini.sh/framework/core/conditions"

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
if err != nil {
    log.Fatal(err)
}
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

## Complete Example

The following YAML shows a pipeline that uses multiple condition types together:

```yaml
pipeline:
  - name: request_classifier
    auto_start: true
    keep_running: true
    processor:
      # Block requests from private networks to admin endpoints
      - if:
          and:
            - network:
                client_ip: private
            - prefix:
                _ctx.request.uri: "/admin"
        then:
          - echo:
              message: "Blocked private network access to admin"

      # Route API errors
      - if:
          and:
            - prefix:
                _ctx.request.uri: "/api/"
            - range:
                _ctx.response.status_code:
                  gte: 400
        then:
          - if:
              range:
                _ctx.response.status_code:
                  lt: 500
            then:
              - echo:
                  message: "API client error"
            else:
              - echo:
                  message: "API server error"

      # Match specific status codes
      - if:
          in:
            _ctx.response.status_code: [301, 302, 307, 308]
        then:
          - echo:
              message: "Redirect detected"

      # Check required fields exist
      - if:
          not:
            exists:
              - _ctx.request.header.X-Request-ID
        then:
          - echo:
              message: "Missing request ID header"

      # Pattern matching on log sources
      - if:
          or:
            - regexp:
                source: "nginx/access.*"
            - regexp:
                source: "apache2/access.*"
        then:
          - echo:
              message: "Web server access log"

      # Queue-based routing
      - if:
          queue_has_lag:
            - "indexing_queue > 5000"
        then:
          - echo:
              message: "Indexing queue has significant lag"
```

## Operator Summary

| Operator | YAML Key | Description |
|----------|----------|-------------|
| Equals | `equals` | Exact value match (string, int, float, bool) |
| Contains | `contains` | Substring match on strings or string arrays |
| Regexp | `regexp` | Regular expression match |
| Prefix | `prefix` | String starts-with check |
| Suffix | `suffix` | String ends-with check |
| In | `in` | Value membership in a list |
| Range | `range` | Numeric range comparison (`gt`, `gte`, `lt`, `lte`) |
| Exists | `exists` | Field existence and non-empty check |
| Length | `length` | Collection/string length equality |
| Network | `network` | IP address network membership |
| AND | `and` | Logical AND (all must match) |
| OR | `or` | Logical OR (any must match) |
| NOT | `not` | Logical negation |
| Queue Has Lag | `queue_has_lag` | Message queue lag detection |
| Consumer Has Lag | `consumer_has_lag` | Consumer group lag detection |
| Cluster Available | `cluster_available` | Elasticsearch cluster availability |
