---
title: "case"
weight: 208
---

# case

| Category | Scope |
|----------|-------|
| Routing | record (sub-chain host) |

Multi-way dispatch: the block declares an ordered list of `branches`, the **first** branch whose match holds runs its `then` sub-chain, and exactly one arm executes. An optional `default` chain handles records no branch matched. It replaces the `if`/`then`/`else` ladder once the same input has to route to three or more destinations. Unlike [`if`](if/), everything — `field`, `branches`, `default` — is nested under the `case` key.

A branch matches on an enumerated value set (`values`, compared against the block's `field`), on a full [condition](../conditions/) (`when`), or on both (AND). Conditions use the same DSL as `if`.

## Structure

```yaml
- case:
    field: <field path>        # optional; required when any branch uses `values`
    branches:
      - values: [<v1>, <v2>]   # match: field value equals any entry
        then:
          - <processor>...
      - when:
          <condition>          # match: any condition operator
        then:
          - <processor>...
    default:                   # optional; runs when no branch matched
      - <processor>...
```

| Key | Description |
|-----|-------------|
| `field` | Field read for `values` matching (record field inside a per-record sub-chain, `_ctx.`-prefixed context field at pipeline level). Required when any branch declares `values`; omit it for a pure condition ladder. |
| `branches` | Ordered list of arms; at least one is required. Evaluation stops at the first match. |
| `branches[].values` | Match when the `field` value equals any entry — the same typed membership test as the [`in`](../conditions/in/) condition. |
| `branches[].when` | Match when the condition holds (`equals`, `regexp`, `range`, `and`, ...). |
| `branches[].then` | Sub-chain for a matching record — a processor or a list of processors, required on every branch. |
| `default` | Optional sub-chain for records no branch matched. Without it, unmatched records pass through unchanged. |

Each branch needs `values` or `when`; declaring both means both must hold. Compile validation rejects a `case` with no branches, a branch without `then`, and a `values` branch in a block that declares no `field`.

## Example

Route by severity, with a prefix match for the `warn*` variants and a catch-all default:

```yaml
- case:
    field: log_level
    branches:
      - values: [error, fatal]
        then:
          - mutate:
              add:
                route: alerting
      - when:
          prefix:
            log_level: warn
        then:
          - mutate:
              add:
                route: warnings
    default:
      - mutate:
          add:
            route: archive
```

Branches without `values` do not need `field`, so the block also works as an ordered condition ladder:

```yaml
- case:
    branches:
      - when:
          and:
            - equals:
                env: prod
            - regexp:
                file.path: '\.json$'
        then:
          - json:
              field: message
      - when:
          exists:
            - file.path
        then:
          - dissect:
              field: message
              pattern: "%{ts} %{level} %{msg}"
```

## Notes

- Exactly one arm runs per record; after it completes the event returns to the main chain. A `terminal` processor inside an arm (e.g. [`drop_event`](drop_event/)) ends the event instead.
- No match and no `default` is not an error — the record continues down the main chain untouched.
- Value resolution follows `if`: inside a per-record sub-chain ([`for_each`](for_each/), [`lane_consumer`](lane_consumer/)) arms see the current record's fields; at pipeline level they read the context through the `_ctx.` prefix.
- Arm chains compile under the enclosing scope, so record-scope contracts keep applying inside nested chains and batch-only processors stay rejected there, exactly as for `if`.
- The arm that ran is recorded in the flow trace as `case:1`, `case:2`, ..., `case:default`, or `case:no-match`.
- Sub-chain processors must be safe for concurrent `Process` calls when the surrounding `for_each` runs with `parallel > 1`.
