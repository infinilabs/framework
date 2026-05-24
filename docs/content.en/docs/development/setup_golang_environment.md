---
weight: 10
title: "Setting Up the Golang Environment"
---

# Setting Up the Golang Environment

Refer the official guide to install Golang: [https://go.dev/doc/install](https://go.dev/doc/install)

## Golang Version

Verify your Go version (1.21+ required):

```bash
➜  ~ go version
go version go1.23.3 darwin/arm64
```

## Directory Setup

Create the necessary directory structure:

```bash
cd ~/go/src/
mkdir -p infini.sh/
```
> Note: The code must be located under your personal directory at ~/go/src/infini.sh.
>
> Other locations are not allowed—this is a strict requirement.

## Cloning Dependencies

Clone the framework repository:
```bash
cd ~/go/src/infini.sh
git clone git@github.com:infinilabs/framework.git
```

> Note: No separate vendor repository is needed. All dependencies are managed via Go modules.

## Cloning Application Code

For example, to work with the Loadgen project:
```bash
cd ~/go/src/infini.sh
git clone git@github.com:infinilabs/loadgen.git
```
## Building the Project

Build the project using the Makefile:
```bash
cd loadgen
make build
```

## Customization Build with Built-in Environments

For example, if you want to expose more debug-level information, such as detecting data races, you can compile a debug build:

```bash
DEV=true make build
```

You can also specify a custom `GOPATH` if needed:

```bash
GOPATH="/Users/<your_username>/go" make build
```

To learn more about the Makefile and its commands, refer to this [Reference](../references/makefile.md).

