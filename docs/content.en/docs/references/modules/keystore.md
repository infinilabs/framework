---
title: "Keystore Module"
weight: 40
---

# Keystore Module

The Keystore module provides secure storage and management of sensitive configuration data such as passwords, API keys, and certificates. It offers a secure API for storing and retrieving key-value pairs with encryption support.

## Features

- Secure storage of sensitive configuration data
- REST API for key-value operations
- Integration with framework configuration system
- Encrypted storage support
- Access control and security

## Configuration

Configure the Keystore module in your YAML configuration:

```yaml
keystore:
  enabled: true
```

## Configuration Parameters

| Parameter | Type | Default | Description |
|-----------|------|---------|-------------|
| `enabled` | boolean | `true` | Enable/disable the keystore module |

## API Endpoints

The Keystore module provides the following API endpoint:

### Key Management

- **POST `/keystore`** - Store a key-value pair in the keystore

Request format:
```json
{
  "key": "database_password", 
  "value": "secret_password_123"
}
```

Response:
```json
{
  "status": "success",
  "message": "Key stored successfully"
}
```

## Usage Examples

### Storing Sensitive Data

```bash
# Store a database password
curl -X POST http://localhost:8080/keystore \
  -H "Content-Type: application/json" \
  -d '{"key": "db_password", "value": "my_secret_password"}'

# Store an API key
curl -X POST http://localhost:8080/keystore \
  -H "Content-Type: application/json" \
  -d '{"key": "elasticsearch_api_key", "value": "your_api_key_here"}'
```

### Integration with Configuration

The keystore integrates with the framework's configuration system, allowing you to reference stored keys in configuration files:

```yaml
elasticsearch:
  - name: "secure_cluster"
    endpoint: "https://elasticsearch.example.com:9200"
    basic_auth:
      username: "elastic"
      password: "${keystore.db_password}"  # References keystore value
```

## Security Features

### Encryption
- All stored values are encrypted at rest
- Keys are securely hashed for lookup
- Memory-safe operations to prevent data leaks

### Access Control
- API endpoints can be protected by authentication
- Audit logging for key access operations
- Role-based access control integration

### Best Practices
1. **Never store keys in plain text** - Always use the keystore for sensitive data
2. **Rotate keys regularly** - Update stored credentials periodically
3. **Monitor access** - Review keystore access logs regularly
4. **Backup securely** - Ensure keystore backups are encrypted
5. **Limit access** - Restrict keystore API access to authorized users

## Integration

The Keystore module integrates with:

- **Global configuration system** - Provides secure value resolution
- **API module** - Exposes REST endpoints for key management
- **Credential system** - Supplies secure credentials to other modules
- **Authentication system** - Protects keystore access

## Data Storage

- Keystore data is stored securely in the framework's data directory
- Files are encrypted using strong encryption algorithms
- Automatic backup and recovery capabilities
- Cross-platform compatibility

## Error Handling

Common error scenarios:

- **Invalid key format** - Keys must follow naming conventions
- **Duplicate keys** - Attempting to store existing keys without overwrite
- **Access denied** - Insufficient permissions for keystore operations
- **Storage failures** - Disk space or permission issues

Example error response:
```json
{
  "status": "error",
  "error": "Key already exists",
  "code": "DUPLICATE_KEY"
}
```

## Troubleshooting

### Common Issues

1. **Module not starting**: Check that `enabled: true` in configuration
2. **API not accessible**: Verify API module is running and accessible
3. **Permission errors**: Check data directory permissions
4. **Storage failures**: Verify available disk space

### Debug Information

- Check module startup logs for initialization errors
- Verify API endpoint registration in startup output
- Monitor keystore access patterns through stats module

## Advanced Usage

### Programmatic Access

```go
import "infini.sh/framework/core/keystore"

// Store a value
err := keystore.Set("my_key", "my_value")
if err != nil {
    log.Error("Failed to store key:", err)
}

// Retrieve a value
value, err := keystore.Get("my_key")
if err != nil {
    log.Error("Failed to retrieve key:", err)
}
```

### Configuration Resolution

The framework automatically resolves keystore references in configuration:

```yaml
# Configuration file
database:
  password: "${keystore.db_password}"
  
# Resolved at runtime to actual keystore value
database:
  password: "actual_secret_password"
```

This provides secure configuration management without exposing sensitive data in configuration files.