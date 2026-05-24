---
weight: 30
title: "Makefile"
---

# Makefile

The framework simplifies managing your application by providing reusable commands and variables. Below is an example of how `Loadgen` utilizes this framework in its Makefile:

Use `Loadgen` for example, here is the project's Makefile looks like:
```shell
➜  loadgen git:(main) cat Makefile
SHELL=/bin/bash

# APP info
APP_NAME := loadgen
APP_VERSION := 1.0.0_SNAPSHOT
APP_CONFIG := $(APP_NAME).yml $(APP_NAME).dsl
APP_EOLDate ?= "2025-12-31T10:10:10Z"
APP_STATIC_FOLDER := .public
APP_STATIC_PACKAGE := public
APP_UI_FOLDER := ui
APP_PLUGIN_FOLDER := proxy

include ../framework/Makefile
```

### Highlights
- Modular and Reusable: By including ../framework/Makefile, this project inherits a suite of predefined commands and variables, reducing redundancy.
- Customizable Variables: Project-specific details (e.g., name, version, config files, and folders) are declared at the top for easy configuration.
- Framework Integration: The framework/Makefile provides consistent functionality across projects, enabling streamlined workflows.

This approach ensures better maintainability and faster setup for new projects.

### Example

To build the `Loadgen` application using the framework, you can run the following command:

```shell
➜  loadgen git:(main) OFFLINE_BUILD=true make build
```
Explanation of the Command:
- OFFLINE_BUILD=true: Enables offline build mode, skipping the `git pull` step for the framework repository.
- make build: Invokes the build target defined in the framework's Makefile, compiling the application.

For development builds with race detection and extra debug info:
```shell
➜  loadgen git:(main) DEV=true make build
```


## Commands

| **Command**                | **Description**                                                                                   | **Dependencies/Notes**                   |
|----------------------------|---------------------------------------------------------------------------------------------------|------------------------------------------|
| `default`                  | Default target, builds the application with race detection                                       | Depends on `build-race`                  |
| `env`                      | Prints environment variables for debugging                                                        | Outputs key paths and repository settings |
| `build`                    | Builds the main application binary                                                                |                                            |
| `build-dev`                | Builds the application with debug symbols and development tags                                   |                                            |
| `build-cmd`                | Builds all binaries in the `cmd` folder                                                          |                                            |
| `cross-build-cmd`          | Cross-compiles binaries for Windows and Linux                                                    |                                            |
| `update-plugins`           | Updates plugin files using plugin discovery tool                                                 | Requires framework discovery binary       |
| `build-race`               | Builds the application with race detection and debug information                                 | Depends on `clean`, `config`, `update-vfs`|
| `tar`                      | Creates a tarball of the application binary and config file                                      | Depends on `build`                        |
| `cross-build`              | Cross-compiles the application for Windows, macOS, and Linux                                     |                                            |
| `build-win`                | Builds the application binary for Windows                                                        |                                            |
| `build-linux-*`            | Builds the application binary for specific Linux architectures (e.g., `amd64`, `arm64`)         |                                            |
| `build-darwin`             | Builds the application binary for macOS                                                          | Supports `amd64` and `arm64`              |
| `build-bsd`                | Builds the application binary for BSD systems                                                   | Supports FreeBSD, NetBSD, and OpenBSD     |
| `all`                      | Cleans, configures, and builds binaries for all supported platforms                              |                                            |
| `all-platform`             | Builds binaries for all platforms, including BSD, Linux, macOS, and Windows                     |                                            |
| `format`                   | Formats all Go source files                                                                     | Uses `go fmt`                             |
| `clean_data`               | Removes data and logs directories                                                                |                                            |
| `clean`                    | Cleans all build artifacts and resets the output directory                                       | Depends on `clean_data`                   |
| `init`                     | Initializes the build environment                                                                | Checks/clones framework repository        |
---


## Variables

| **Variable**              | **Description**                                                                                      | **Default Value**                        |
|---------------------------|------------------------------------------------------------------------------------------------------|------------------------------------------|
| `APP_NAME`                | Application name                                                                                    | `framework`                              |
| `APP_VERSION`             | Application version                                                                                 | `1.0.0_SNAPSHOT`                         |
| `APP_CONFIG`              | Configuration file name                                                                             | `$(APP_NAME).yml`                        |
| `APP_EOLDate`             | End-of-life date for the application                                                                | `"2023-12-31T10:10:10Z"`                 |
| `APP_STATIC_FOLDER`       | Path to static folder                                                                               | `.public`                                |
| `APP_STATIC_PACKAGE`      | Static package name                                                                                 | `public`                                 |
| `APP_UI_FOLDER`           | UI folder path                                                                                      | `ui`                                     |
| `APP_PLUGIN_FOLDER`       | Plugins folder path                                                                                 | `plugins`                                |
| `APP_PLUGIN_PKG`          | Plugins package name                                                                                | `$(APP_PLUGIN_FOLDER)`                   |
| `APP_NEED_CGO`            | Determines if CGO is required (0 = disabled, 1 = enabled)                                           | `0`                                      |
| `VERSION`                 | Release version from the environment                                                                |                                          |
| `GOPATH`                  | Go workspace path. Can be overridden to use a custom location.                                      | `~/go`                                   |
| `BUILD_NUMBER`            | Build number                                                                                        | `001`                                    |
| `DEV`                     | Enables development mode (adds `-tags dev` to build). Set to any non-empty value to enable.         |                                          |
| `OFFLINE_BUILD`           | Skips `git pull` for the framework during `init`. Set to any non-empty value to enable.             |                                          |
| `GO`                      | Go command                                                                                          | `go`                                     |
| `FRAMEWORK_FOLDER`        | Path to INFINI Framework folder                                                                     | `$(INFINI_BASE_FOLDER)/framework`        |
| `FRAMEWORK_REPO`          | Framework repository URL                                                                            | `https://github.com/infinilabs/framework.git` |
| `FRAMEWORK_BRANCH`        | Git branch for the framework                                                                        | `main`                                   |

---


### Notes

 - **Go Modules**: All dependencies are managed via Go modules (`go.mod`). No external vendor repository is required.
 - **Cross-Platform Builds**: Targets like `build-linux` and `build-darwin` compile binaries for multiple architectures, ensuring compatibility across platforms.
 - **Plugin Updates**: Plugins are dynamically discovered and updated using a tool within the framework. Ensure `plugin-discovery` exists and is built.
 - **Environment Variables**: Many configurations (e.g., `GOPATH`, `VERSION`, `EOL`) can be overridden via environment variables for flexibility.

