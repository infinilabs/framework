---
title: "network"
weight: 90
---

# network

| Kind | Accepts |
|------|---------|
| Value operator | one field and named networks or CIDRs |

Tests whether an IP address field belongs to a specific network. Accepts named network identifiers or CIDR notation; multiple networks can be given as a list, matching when the address belongs to any of them.

## Parameters

| Parameter | Type | Description |
|-----------|------|-------------|
| `loopback` | named | Loopback addresses (e.g. `127.0.0.1`, `::1`). |
| `private` | named | RFC 1918 (IPv4) and RFC 4193 (IPv6) private addresses. |
| `public` | named | Any address that is not local or private. |
| `global_unicast` / `unicast` | named | Global unicast addresses. |
| `link_local_unicast` | named | Link-local unicast addresses. |
| `multicast` | named | Multicast addresses. |
| `link_local_multicast` | named | Link-local multicast addresses. |
| `interface_local_multicast` | named | Interface-local multicast addresses. |
| `unspecified` | named | The unspecified address (`0.0.0.0` or `::`). |
| CIDR | string | e.g. `192.168.1.0/24`. |

## Examples

```yaml
# Named network
network:
  client_ip: private

# CIDR notation
network:
  source.ip: "192.168.1.0/24"

# Any of several networks
network:
  client_ip: ["private", "loopback"]
```

## Notes

Inside a per-record sub-chain (e.g. `for_each`), field names resolve against the current
record's own attributes (`file.path`, `log_level`, ...); at pipeline level they resolve against
the pipeline context through the `_ctx.` prefix. See the Conditions reference for details.
