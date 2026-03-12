---
title: "Keystore"
weight: 60
---

# Keystore

The INFINI Framework provides a secure keystore for managing sensitive configuration values such as passwords, API keys, certificates, and other secrets. Secrets are stored in an AES-256 encrypted file on disk and can be referenced in configuration files using variable substitution.

## Overview

The keystore offers three ways to manage secrets:

- **CLI Commands**: Add, list, and remove secrets from the command line.
- **Configuration Reference**: Use `$[[keystore.<key>]]` syntax to inject secrets into YAML configuration files.

Secrets are stored under the application's data directory in a `.keystore/` folder containing an encrypted store file (`ks`) and an auto-generated encryption key (`key`).

## CLI Commands

The keystore CLI is accessed via the `keystore` subcommand of your application binary.

### Usage

```shell
<app> keystore <command> [<args>]
```

### Available Commands

| Command  | Description                   |
|----------|-------------------------------|
| `add`    | Add or update a secret        |
| `list`   | List all secret keys          |
| `remove` | Remove a secret by key        |

### Adding a Secret

#### Interactive Mode

When run without `--stdin`, the command prompts for the secret value with hidden input (no echo):

```shell
<app> keystore add my_password
Enter value for my_password:
success
```

> **Note:** Interactive mode uses the terminal's canonical input, which is limited to approximately 1024 bytes by the operating system. For larger values or values containing newlines, use `--stdin` mode instead.

#### Stdin Mode

Use `--stdin` to pipe a value from standard input. This mode supports **arbitrary length** values and **multi-line** content (e.g., certificates, private keys):

```shell
# From a simple string
echo -n "my_secret_value" | <app> keystore add my_key --stdin

# From a file (preserves newlines)
cat /path/to/cert.pem | <app> keystore add tls_cert --stdin

# Using a heredoc
<app> keystore add my_key --stdin <<EOF
line1
line2
line3
EOF
```

Trailing `\r\n` or `\n` characters are automatically trimmed, but embedded newlines within the value are preserved.

#### Flags

| Flag       | Description                                            |
|------------|--------------------------------------------------------|
| `--stdin`  | Read the secret value from standard input instead of the terminal prompt |
| `--force`  | Overwrite an existing key without prompting for confirmation |

#### Overwriting an Existing Secret

In interactive mode, you will be prompted to confirm the overwrite:

```shell
<app> keystore add my_password
Secret my_password already exists, Overwrite? [y/N]: y
Enter value for my_password:
success
```

With `--stdin`, use `--force` to overwrite without confirmation:

```shell
echo -n "new_value" | <app> keystore add my_password --stdin --force
```

### Listing Secrets

List all keys stored in the keystore:

```shell
<app> keystore list
```

Example output:

```
my_password
tls_cert
api_key
```

Only key names are displayed; secret values are never printed.

### Removing a Secret

Remove a secret by its key name:

```shell
<app> keystore remove my_password
```

## Using Secrets in Configuration

Secrets stored in the keystore can be referenced in YAML configuration files using the `$[[keystore.<key>]]` variable syntax:

```yaml
elasticsearch:
  - name: production
    endpoint: https://localhost:9200
    basic_auth:
      username: admin
      password: $[[keystore.my_password]]
```

When the configuration is loaded, `$[[keystore.my_password]]` is resolved to the secret value stored under the key `my_password`.

## Storage Location

By default, the keystore is stored in the application's data directory:

```
<data_dir>/.keystore/
├── key    # Auto-generated AES encryption key
└── ks     # Encrypted secrets store
```

### Custom Path

Set the `KEYSTORE_PATH` environment variable to override the storage location:

```shell
export KEYSTORE_PATH=/secure/path/to/keystore
<app> keystore add my_key
```


## Security Considerations

- The keystore file is encrypted using **AES-256-GCM** with a key derived via **PBKDF2** (SHA-512, 10,000 iterations).
- The encryption key file (`key`) should be protected with restrictive file permissions (for example, `0600`).
- Secret values are never logged or printed to the console.
- The keystore supports file watching — changes are automatically reloaded at runtime.
