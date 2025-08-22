---
weight: 10
title: "Setting Up the Golang Environment"
---

# Setting Up the Golang Environment

Refer the official guide to install Golang: [https://go.dev/doc/install](https://go.dev/doc/install)

## Golang Version

Verify your Go version:

```bash
➜  loadgen git:(master) ✗ go version
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

Clone the required dependency repositories:
```bash
cd ~/go/src/infini.sh
git@github.com:infinilabs/framework.git
```

## Cloning Application Code

For example, to work with the Loadgen project:
```bash
cd ~/go/src/infini.sh
git@github.com:infinilabs/loadgen.git
```
## Building the Project

Build the project using the Makefile:
```bash
cd loadgen
make
```
The make command will automatically download the required dependency repositories.

## Customization Build with Built-in Environments

For example, if you want to expose more debug-level information, such as detecting data races, you can compile a debug build. You may also specify the `GOPATH` as needed. Use the following command:

```bash
DEV=true GOPATH="/Users/<Replace_with_your_username>/go" make build
```
To learn more about the Makefile and its commands, refer to this [Reference](../references/makefile.md).

