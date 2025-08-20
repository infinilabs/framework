---
title: "HTTP Plugin"
weight: 20
---

# HTTP Plugin

The HTTP plugin provides HTTP client functionality for sending requests to external services as part of data processing pipelines. It supports advanced features like rate limiting, connection pooling, compression, and retry logic.

## Features

- HTTP/HTTPS request processing in pipelines
- Configurable request methods, headers, and paths
- Rate limiting and connection pooling
- Request compression and response handling
- Retry logic with configurable delays
- TLS configuration support
- Template-based URL and path construction
- Basic authentication support

## Configuration

Configure the HTTP plugin as a processor in your pipeline:

```yaml
pipelines:
  - name: "http_output"
    processors:
      - http:
          message_field: "messages"
          schema: "https"
          hosts: ["api.example.com"]
          method: "POST"
          path: "/webhook"
          headers:
            Content-Type: "application/json"
            User-Agent: "INFINI-Framework/1.0"
          basic_auth:
            username: "api_user"
            password: "api_password"
          
          # Performance Settings
          max_sending_qps: 100
          max_connection_per_node: 10
          max_response_size: 1048576  # 1MB
          timeout: "30s"
          
          # Retry Configuration
          max_retry_times: 3
          retry_delay_in_ms: 1000
          
          # Compression
          compress: true
          compression_threshold: 1024
          
          # Validation
          valid_status_code: [200, 201, 202]
```

## Configuration Parameters

### Basic Configuration

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `message_field` | string | `"messages"` | Field containing message data to send |
| `schema` | string | `"http"` | Protocol schema (http/https) |
| `hosts` | []string | `[]` | Target host addresses |
| `method` | string | `"GET"` | HTTP method (GET, POST, PUT, DELETE, etc.) |
| `path` | string | `"/"` | Request path (supports templates) |
| `headers` | map | `{}` | Custom HTTP headers |

### Authentication

| Parameter | Type | Description |
|-----------|------|-------------|
| `basic_auth.username` | string | Basic authentication username |
| `basic_auth.password` | string | Basic authentication password |
| `tls` | TLSConfig | TLS/SSL configuration |

### Performance Configuration

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `max_sending_qps` | int | `0` | Maximum requests per second (0 = unlimited) |
| `max_connection_per_node` | int | `10` | Maximum connections per host |
| `max_response_size` | int | `1048576` | Maximum response body size in bytes |
| `timeout` | duration | `"10s"` | Overall request timeout |
| `read_timeout` | duration | `"10s"` | Read timeout |
| `write_timeout` | duration | `"10s"` | Write timeout |
| `read_buffer_size` | int | `4096` | Read buffer size |
| `write_buffer_size` | int | `4096` | Write buffer size |

### Connection Management

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `max_conn_wait_timeout` | duration | `"1s"` | Maximum wait time for connection |
| `max_idle_conn_duration` | duration | `"90s"` | Maximum idle connection duration |
| `max_conn_duration` | duration | `"0s"` | Maximum connection duration (0 = unlimited) |

### Retry and Error Handling

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `max_retry_times` | int | `3` | Maximum number of retry attempts |
| `retry_delay_in_ms` | int | `1000` | Delay between retries in milliseconds |
| `valid_status_code` | []int | `[200, 201]` | HTTP status codes considered successful |

### Compression

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `compress` | boolean | `false` | Enable request body compression |
| `compression_threshold` | int | `1024` | Minimum size in bytes to enable compression |

## Path Templates

The HTTP plugin supports templated paths for dynamic URL construction:

```yaml
http:
  path: "/api/v1/users/{user_id}/events"
  # Will be resolved at runtime using message context
```

Template variables can be resolved from:
- Message content
- Pipeline context
- Global configuration

## Usage Examples

### Simple Webhook
```yaml
processors:
  - http:
      schema: "https"
      hosts: ["webhook.example.com"]
      method: "POST"
      path: "/webhook"
      headers:
        Content-Type: "application/json"
```

### High-Performance Output
```yaml
processors:
  - http:
      schema: "https"
      hosts: ["api1.example.com", "api2.example.com"]
      method: "POST"
      path: "/bulk"
      max_sending_qps: 1000
      max_connection_per_node: 50
      compress: true
      max_retry_times: 5
```

### Elasticsearch Integration
```yaml
processors:
  - http:
      schema: "https"
      hosts: ["elasticsearch.example.com:9200"]
      method: "POST"
      path: "/logs/_doc"
      basic_auth:
        username: "${elasticsearch.username}"
        password: "${elasticsearch.password}"
      headers:
        Content-Type: "application/json"
```

## Integration

The HTTP plugin integrates with:

- **Pipeline system** - Processes messages in data pipelines
- **Rate limiting** - Controls request rates to prevent overload
- **Connection pooling** - Manages HTTP connections efficiently
- **Template system** - Supports dynamic URL construction
- **Configuration system** - Supports variable resolution

## Performance Optimization

### Connection Management
- Use connection pooling for high-throughput scenarios
- Adjust `max_connection_per_node` based on target capacity
- Configure appropriate timeouts for your network conditions

### Rate Limiting
- Set `max_sending_qps` to respect API rate limits
- Use multiple hosts for load distribution
- Monitor response times and adjust rates accordingly

### Compression
- Enable compression for large payloads
- Adjust `compression_threshold` based on your data size
- Monitor CPU usage vs. bandwidth savings

## Monitoring

The HTTP plugin provides metrics through the stats system:
- Request counts and success rates
- Response times and error rates
- Connection pool statistics
- Retry attempt counts

## Error Handling

### Retry Logic
- Automatic retries for transient failures
- Configurable retry delays and maximum attempts
- Exponential backoff support

### Status Code Validation
- Configure acceptable HTTP status codes
- Automatic error handling for invalid responses
- Detailed error logging for troubleshooting

## Troubleshooting

### Common Issues

1. **Connection timeouts**
   - Increase timeout values
   - Check network connectivity
   - Verify target service availability

2. **Rate limiting errors**
   - Reduce `max_sending_qps`
   - Implement exponential backoff
   - Check target API rate limits

3. **Authentication failures**
   - Verify credentials configuration
   - Check authentication method requirements
   - Monitor authentication token expiration

### Debug Configuration
```yaml
http:
  # Enable detailed logging
  debug: true
  # Log request/response details
  log_requests: true
```

This configuration provides detailed request and response logging for troubleshooting HTTP plugin issues.