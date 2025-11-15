---
title: "Http Client"
weight: 50
---

# HTTP Client Configuration

The `http_client` section defines configurations for HTTP clients, where each key represents a unique client profile. The `default` key is a special profile that serves as the fallback configuration when no specific profile is specified. Additional profiles can be added and accessed dynamically.

## Sample Configuration

```yaml
http_client:
  default:
    proxy:
      enabled: true
      default_config:
        http_proxy: http://127.0.0.1:7890
        socket5_proxy: socks5://127.0.0.1:7890
      override_system_proxy_env: true # Override system proxy environment settings
      permitted:
        - "google.com"
      denied:
        - "localhost"
        - "infinilabs.com"
        - "api.coco.rs"
      domains:
        "github.com":
          http_proxy: http://127.0.0.1:7890
          socket5_proxy: socks5://127.0.0.1:7890
  custom_profile:
    proxy:
      enabled: false
  gitlab_p12:
    tls:
      enabled: true
      cert_file: /fullpath/test-client.p12
      cert_password: "12345"
```

# Parameters

## HTTP Client Configuration: `http_client`
| Name   | Type                       | Description                                                                 |
|--------|----------------------------|-----------------------------------------------------------------------------|
| `key`  | map[string]HTTPClientConfig | Each key represents a named HTTP client configuration. For example, `default` or `custom`. |
| `default` | HTTPClientConfig         | The default configuration used as a fallback when no specific configuration is specified. |

---

## HTTP Client Config: `HTTPClientConfig`
| Name                        | Type      | Description                                                                 |
|-----------------------------|-----------|-----------------------------------------------------------------------------|
| `proxy`                     | ProxyConfig | Configuration for proxy usage, including domain rules and proxy settings.   |
| `timeout`                   | string    | The overall timeout for HTTP requests.                                      |
| `dial_timeout`              | string    | The timeout for establishing connections.                                   |
| `read_timeout`              | string    | The timeout for reading data from the connection.                          |
| `write_timeout`             | string    | The timeout for writing data to the connection.                            |
| `read_buffer_size`          | int       | The size of the read buffer.                                               |
| `write_buffer_size`         | int       | The size of the write buffer.                                              |
| `tls`                | TLSConfig | Configuration for TLS settings.                                            |
| `max_connection_per_host`   | int       | The maximum number of connections per host.                                |

---

## Proxy Configuration: `ProxyConfig`
| Name                        | Type    | Description                                                                 |
|-----------------------------|---------|-----------------------------------------------------------------------------|
| `enabled`                   | boolean | Enables or disables the use of a proxy.                                    |
| `default_config`                    | ProxyDetails | Default proxy settings, including HTTP and SOCKS5 proxies.                 |
| `override_system_proxy_env` | boolean | Whether to override system-wide proxy environment variables.                |
| `permitted`                 | list    | List of domains allowed to use the proxy.                                  |
| `denied`                    | list    | List of domains denied from using the proxy.                               |
| `domains`                    | map    | Proxy settings per domain.                               |

---

## Proxy Details: `ProxyDetails`
| Name                        | Type    | Description                                                                 |
|-----------------------------|---------|-----------------------------------------------------------------------------|
| `http_proxy`                | string  | URL of the HTTP proxy.                                                     |
| `socket5_proxy`             | string  | URL of the SOCKS5 proxy.                                                   |
| `using_proxy_env`           | boolean | Whether to use system environment proxy settings (e.g., `HTTP_PROXY`).      |

---

## TLS Configuration: `TLSConfig`
| Name                        | Type      | Description                                                                 |
|-----------------------------|-----------|-----------------------------------------------------------------------------|
| `enabled`                   | boolean   | Enables or disables TLS/SSL for the connection.                            |
| `cert_file`                 | string    | Path to the TLS certificate file. Support PKCS#12 and PEM file.     |
| `cert_password`             | string    | Password for the TLS certificate file (if encrypted).                      |
| `key_file`                  | string    | Path to the TLS private key file.                                          |
| `ca_file`                   | string    | Path to the Certificate Authority (CA) certificate file.                   |
| `skip_insecure_verify`      | boolean   | Skip TLS certificate verification (insecure - use with caution).           |
| `default_domain`            | string    | Default domain for TLS certificate generation.                             |
| `skip_domain_verify`        | boolean   | Skip domain verification for AutoIssue certificates.                       |
| `client_session_cache_size` | int       | Size of the client TLS session cache.                                      |
