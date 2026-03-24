---
title: "Configuration Management"
weight: 5
---
# Configuration Management

The INFINI Framework provides a comprehensive configuration management system built on top of `ucfg`. It supports YAML-based configuration files, struct unpacking with `config` tags, environment variable interpolation, keystore secret references, config file watching, and multi-file config merging. Modules and plugins use a consistent pattern to declare, load, and apply their configuration.

## Overview

Configuration in the INFINI Framework follows a layered approach:

1. **Application config file** (e.g., `myapp.yml`) — defines paths, API settings, modules, plugins, logging, and resource limits.
2. **Module/plugin configs** — each module reads its own section from the application config.
3. **Environment variables** — values can be injected at runtime using `${ENV_VAR}` syntax.
4. **Keystore secrets** — sensitive values are referenced with `$[[keystore.secret_name]]` and resolved from an encrypted keystore.
5. **Config file watching** — changes to configuration files on disk can trigger reload callbacks.

## The `Config` Type

The core configuration type wraps `ucfg.Config` and is defined in the `core/config/` package:

```go
type Config = ucfg.Config
```

### Creating Configs

| Function | Description |
|----------|-------------|
| `NewConfig()` | Returns an empty `*Config`. |
| `NewConfigFrom(v interface{})` | Creates a `*Config` from a Go map, struct, or other compatible value. |
| `NewConfigWithYAML(yaml []byte, source string)` | Parses raw YAML bytes into a `*Config`. The `source` string is used in error messages. |

```go
// Empty config
cfg := config.NewConfig()

// From a Go map
cfg, err := config.NewConfigFrom(map[string]interface{}{
    "host": "localhost",
    "port": 9200,
})

// From raw YAML
yamlBytes := []byte(`
host: localhost
port: 9200
`)
cfg, err := config.NewConfigWithYAML(yamlBytes, "inline")
```

## Loading Configuration Files

Use `LoadFile` and `LoadFiles` to read YAML configuration from disk:

```go
// Load a single file
cfg, err := config.LoadFile("/etc/myapp/myapp.yml")

// Load multiple files
cfgs, err := config.LoadFiles(
    "/etc/myapp/base.yml",
    "/etc/myapp/override.yml",
)
```

`LoadFile` accepts optional `Option` arguments for controlling parsing behavior. `LoadFiles` returns a slice of `*Config` values, one per file, which can be merged together.

## Unpacking Config to Structs

Configuration values are unpacked into Go structs using the `Unpack` method. Struct fields are mapped to YAML keys via `config` tags:

```go
type MyConfig struct {
    Enabled  bool   `config:"enabled"`
    Host     string `config:"host"`
    Port     int    `config:"port"`
    Timeout  string `config:"timeout"`
}

cfg := MyConfig{}
err := config.Unpack(&cfg)
```

### Tag Rules

- The `config` tag name maps directly to the YAML key.
- Nested structs map to nested YAML objects.
- Unexported fields and fields without a `config` tag are ignored.
- Default values can be set on the struct before calling `Unpack`; only keys present in the config will overwrite them.

```go
type ServerConfig struct {
    Network NetworkConfig `config:"network"`
    TLS     TLSConfig     `config:"tls"`
}

type NetworkConfig struct {
    Host string `config:"host"`
    Port int    `config:"port"`
}

type TLSConfig struct {
    Enabled  bool   `config:"enabled"`
    CertFile string `config:"cert_file"`
    KeyFile  string `config:"key_file"`
}
```

The corresponding YAML:

```yaml
network:
  host: 0.0.0.0
  port: 9000
tls:
  enabled: true
  cert_file: /etc/certs/server.crt
  key_file: /etc/certs/server.key
```

## Application Configuration Structure

The main application configuration file defines system-wide settings. The following sections are supported:

### Path Settings

```yaml
path:
  data: data
  log: log
  config: config
```

| Field | Description |
|-------|-------------|
| `data` | Directory for application data files (e.g., keystore, state). |
| `log` | Directory for log files. |
| `config` | Directory for additional configuration files. |

### API Server

```yaml
api:
  enabled: true
  network:
    host: 0.0.0.0
    port: 9000
```

| Field | Description |
|-------|-------------|
| `enabled` | Whether the built-in HTTP API server is active. |
| `network.host` | Bind address for the API server. |
| `network.port` | Listen port for the API server. |

### Modules

```yaml
modules:
  - name: elasticsearch
    enabled: true
  - name: pipeline
    enabled: true
```

Each entry names a registered module and controls whether it is enabled at startup. Modules may define additional keys under their entry that are parsed during the module's `Setup()` phase.

### Plugins

```yaml
plugins:
  - name: badger
    enabled: true
  - name: redis
    enabled: false
```

Plugins follow the same pattern as modules. They are registered separately and started after all system modules.

### Logging

```yaml
logging:
  level: info
  log_level: info
```

| Field | Description |
|-------|-------------|
| `level` | Global log level (`debug`, `info`, `warn`, `error`). |
| `log_level` | Alias for `level`. When both are set, `log_level` takes precedence. |

### Resource Limits

```yaml
resource_limit:
  mem:
    max_memory_in_bytes: 0
  cpu:
    max_num_of_cpus: 0
```

| Field | Description |
|-------|-------------|
| `mem.max_memory_in_bytes` | Maximum memory usage in bytes. `0` means unlimited. |
| `cpu.max_num_of_cpus` | Maximum number of OS threads. `0` means use all available CPUs. |

## Environment Variable Interpolation

Configuration values can reference environment variables using `${ENV_VAR}` syntax. Variables are resolved when the configuration file is loaded:

```yaml
elasticsearch:
  - name: production
    endpoint: ${ES_ENDPOINT}
    basic_auth:
      username: ${ES_USER}
      password: ${ES_PASSWORD}
```

```shell
export ES_ENDPOINT=https://es.example.com:9200
export ES_USER=admin
export ES_PASSWORD=changeme
./myapp
```

If an environment variable is not set, the reference is left as-is or resolved to an empty string depending on the configuration options in use. This mechanism is useful for containerized deployments where configuration is injected through the environment.

## Keystore Secret References

For sensitive values that should not be stored in plain text, the framework supports references to an encrypted keystore using the `$[[keystore.secret_name]]` syntax:

```yaml
elasticsearch:
  - name: production
    endpoint: https://localhost:9200
    basic_auth:
      username: admin
      password: $[[keystore.es_password]]
```

Secrets are managed with the `keystore` CLI subcommand (see [Keystore]({{< relref "keystore" >}}) for details). At load time, `$[[keystore.es_password]]` is replaced with the decrypted value of the `es_password` key from the keystore.

Keystore references and environment variables can be used together in the same configuration file. Keystore references are resolved separately from environment variable interpolation.

## Config File Watching

The framework can watch configuration files for changes and invoke a callback when a modification is detected. This is useful for hot-reloading configuration without restarting the application:

```go
config.NotifyOnConfigChange(func(ev fsnotify.Event) {
    log.Infof("config file changed: %s", ev.Name)
    // Re-read configuration and apply changes
})
```

The watcher uses filesystem notifications (`fsnotify`) and fires the callback for create, write, and rename events on the configuration file. Ensure that the callback is safe for concurrent execution if your application may trigger multiple rapid file changes.

## Module Configuration Pattern

Modules and plugins follow a consistent pattern for loading their configuration section from the application config:

```go
type MyModule struct {}

type MyModuleConfig struct {
    Enabled  bool   `config:"enabled"`
    Host     string `config:"host"`
    Port     int    `config:"port"`
    Timeout  string `config:"timeout"`
}

func (module *MyModule) Name() string {
    return "my_module"
}

func (module *MyModule) Setup() {
    cfg := MyModuleConfig{
        // Set defaults
        Enabled: true,
        Host:    "127.0.0.1",
        Port:    9200,
        Timeout: "30s",
    }
    // Parse the "my_module" section from the application config
    env.ParseConfig("my_module", &cfg)

    if !cfg.Enabled {
        return
    }
    // Use cfg.Host, cfg.Port, cfg.Timeout ...
}
```

The corresponding application config:

```yaml
my_module:
  enabled: true
  host: 10.0.0.1
  port: 9300
  timeout: 60s
```

`env.ParseConfig` reads the named section from the global application config and unpacks it into the provided struct. Fields not present in the YAML retain their default values set before the call.

## Merging Configs

Multiple `*Config` values can be merged into a single unified config using `MergeConfigs`. Later configs override keys from earlier ones:

```go
base, err := config.LoadFile("base.yml")
override, err := config.LoadFile("override.yml")

merged, err := config.MergeConfigs(base, override)
```

This is useful for layered configuration strategies such as:

- A **base** config with shared defaults.
- An **environment-specific** override (e.g., `production.yml`, `staging.yml`).
- A **local** override for developer machines.

Keys present in later configs overwrite the same keys from earlier configs. Keys only present in earlier configs are preserved.

## Complete Example

Below is a full application configuration file demonstrating all major sections:

```yaml
# Application paths
path:
  data: /var/lib/myapp
  log: /var/log/myapp
  config: /etc/myapp

# API server
api:
  enabled: true
  network:
    host: 0.0.0.0
    port: 9000

# Modules
modules:
  - name: elasticsearch
    enabled: true
  - name: pipeline
    enabled: true

# Plugins
plugins:
  - name: badger
    enabled: true

# Logging
logging:
  level: info

# Resource limits
resource_limit:
  mem:
    max_memory_in_bytes: 2147483648  # 2 GB
  cpu:
    max_num_of_cpus: 4

# Module-specific settings
elasticsearch:
  - name: production
    endpoint: ${ES_ENDPOINT}
    basic_auth:
      username: admin
      password: $[[keystore.es_password]]
```

And the corresponding module code that consumes the configuration:

```go
package main

import (
    "infini.sh/framework/core/config"
    "infini.sh/framework/core/env"
    "infini.sh/framework/core/module"
)

type AppModule struct{}

type AppModuleConfig struct {
    Enabled bool   `config:"enabled"`
    Host    string `config:"host"`
    Port    int    `config:"port"`
}

func (m *AppModule) Name() string  { return "app" }
func (m *AppModule) Stop() error   { return nil }
func (m *AppModule) Start() error  { return nil }

func (m *AppModule) Setup() {
    cfg := AppModuleConfig{
        Enabled: true,
        Host:    "127.0.0.1",
        Port:    8080,
    }
    env.ParseConfig("app", &cfg)
    // cfg now contains merged values from defaults + YAML
}

func init() {
    module.RegisterSystemModule(&AppModule{})
}
```
