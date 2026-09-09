---
title: "community_id"
weight: 3002
---

# community_id

| Category | Scope |
|----------|-------|
| Enrichment | record |

## Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `source_ip` | string | "source.ip" | Network field holding the source IP. |
| `source_port` | string | "source.port" | Network field holding the source port. |
| `destination_ip` | string | "destination.ip" | Network field holding the destination IP. |
| `destination_port` | string | "destination.port" | Network field holding the destination port. |
| `iana_number` | string | "network.iana_number" | Transport protocol's IANA number. |
| `transport` | string | "network.transport" | TLS transport mode of the connection. |
| `icmp_type` | string | "icmp.type" | Network field holding the ICMP type. |
| `icmp_code` | string | "icmp.code" | Network field holding the ICMP code. |

## Example

```yaml
processor:
  - community_id:
      source_ip: "source_ip"
      source_port: "source_port"
      destination_ip: "destination_ip"
      destination_port: "destination_port"
      iana_number: "iana_number"
```
