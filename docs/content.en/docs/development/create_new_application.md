---
weight: 30
title: "Create New Application"
---

# Create New Application

Let's use the `NewAPP` as the new project for example.

## Create the project layout

Use the name `new_app` as the project id, and create the project folder as below:
```shell
cd ~/go/src/infini.sh/
mkdir new_app
```
> Note: Ensure that `new_app` is located in the same directory as the `framework` folder. This structure is required for the Makefile to function correctly.

## Create the main file

Create a empty `main.go` file, and paste the code as below:

```shell
package main

import (
        "infini.sh/framework"
        "infini.sh/framework/core/module"
        "infini.sh/framework/core/util"
        "infini.sh/framework/modules/api"
        "infini.sh/new_app/config"
)

func main() {

        terminalHeader := ("     __               _      ___  ___ \n")
        terminalHeader += ("  /\\ \\ \\_____      __/_\\    / _ \\/ _ \\\n")
        terminalHeader += (" /  \\/ / _ \\ \\ /\\ / //_\\\\  / /_)/ /_)/\n")
        terminalHeader += ("/ /\\  /  __/\\ V  V /  _  \\/ ___/ ___/ \n")
        terminalHeader += ("\\_\\ \\/ \\___| \\_/\\_/\\_/ \\_/\\/   \\/     \n\n")

        terminalFooter := ("Goodbye~")
        app := framework.NewApp("new_app", "Make a golang application is such easy~.",
                util.TrimSpaces(config.Version), util.TrimSpaces(config.BuildNumber), util.TrimSpaces(config.LastCommitLog), util.TrimSpaces(config.BuildDate), util.TrimSpaces(config.EOLDate), terminalHeader, terminalFooter)
        app.IgnoreMainConfigMissing()
        app.Init(nil)
        defer app.Shutdown()
        if app.Setup(func() {
                module.RegisterSystemModule(&api.APIModule{})
                module.Start()
        }, func() {
        }, nil) {
                app.Run()
        }
}
```

> You may use this online tool to generate your beauty ASCII based terminal header: [http://patorjk.com/software/taag/#p=display&h=2&v=1&f=Ogre&t=NewAPP](http://patorjk.com/software/taag/#p=display&h=2&v=1&f=Ogre&t=NewAPP)

## Create the config file
```
touch new_app.yml
```
## Create the makefile

create a empty `Makefile`, and paste the code as below:

```shell
SHELL=/bin/bash

# APP info
APP_NAME := new_app
APP_VERSION := 1.0.0_SNAPSHOT
APP_CONFIG := $(APP_NAME).yml
APP_EOLDate ?= "2025-12-31T10:10:10Z"
APP_STATIC_FOLDER := .public
APP_STATIC_PACKAGE := public
APP_UI_FOLDER := ui
APP_PLUGIN_FOLDER := plugins
PREFER_MANAGED_VENDOR=fase

include ../framework/Makefile
```

## Build the application
```shell
➜  new_app OFFLINE_BUILD=true make build
building new_app 1.0.0_SNAPSHOT main
/Users/medcl/go/src/infini.sh/new_app
framework path:  /Users/medcl/go/src/infini.sh/framework
fatal: not a git repository (or any of the parent directories): .git
update generated info
update configs
(cd ../framework/  && make update-plugins) || true # build plugins in framework
GOPATH=~/go:~/go/src/infini.sh/framework/../vendor/ CGO_ENABLED=0 GRPC_GO_REQUIRE_HANDSHAKE=off  GO15VENDOREXPERIMENT="1" GO111MODULE=off go build -a  -gcflags=all="-l -B"  -ldflags '-static' -ldflags='-s -w' -gcflags "-m"  --work  -o /Users/medcl/go/src/infini.sh/new_app/bin/new_app
WORK=/var/folders/j5/qd4qt3n55dz053d93q2mswfr0000gn/T/go-build435280758
# infini.sh/new_app
./main.go:17:9: can inline main.deferwrap1
./main.go:21:12: can inline main.func2
./main.go:18:22: func literal does not escape
./main.go:19:45: &api.APIModule{} escapes to heap
./main.go:21:12: func literal escapes to heap
restore generated info
```

## Run the application
```shell
➜  new_app git:(main) ✗ ./bin/new_app
     __               _      ___  ___
  /\ \ \_____      __/_\    / _ \/ _ \
 /  \/ / _ \ \ /\ / //_\\  / /_)/ /_)/
/ /\  /  __/\ V  V /  _  \/ ___/ ___/
\_\ \/ \___| \_/\_/\_/ \_/\/   \/

[NEW_APP] Make a golang application is such easy~.
[NEW_APP] 1.0.0_SNAPSHOT#001, 2024-12-16 06:15:10, 2025-12-31 10:10:10, HEAD
[12-16 14:15:19] [INF] [env.go:203] configuration auto reload enabled
[12-16 14:15:19] [INF] [env.go:209] watching config: /Users/medcl/go/src/infini.sh/new_app/config
[12-16 14:15:19] [INF] [app.go:311] initializing new_app, pid: 64426
[12-16 14:15:19] [INF] [app.go:312] using config: /Users/medcl/go/src/infini.sh/new_app/new_app.yml
[12-16 14:15:19] [INF] [api.go:214] local ips: 192.168.3.17
[12-16 14:15:19] [INF] [api.go:312] api server listen at: http://0.0.0.0:2900
[12-16 14:15:19] [INF] [module.go:159] started module: api
[12-16 14:15:19] [INF] [module.go:184] all modules are started
[12-16 14:15:19] [INF] [instance.go:101] workspace: /Users/medcl/go/src/infini.sh/new_app/data/new_app/nodes/ctfs8hbq50kevmkb3m6g
[12-16 14:15:19] [INF] [app.go:537] new_app is up and running now.
^C
[NEW_APP] got signal: interrupt, start shutting down
[12-16 14:15:23] [INF] [module.go:213] all modules are stopped
[12-16 14:15:23] [INF] [app.go:410] new_app now terminated.
[NEW_APP] 1.0.0_SNAPSHOT, uptime: 4.13334s

Goodbye~
```

## Conclusion

By leveraging the INFINI Framework, creating a Go application becomes significantly simpler and more efficient.
The framework provides built-in commands and modules, streamlining the development process and enabling you to focus on building your application's core functionality.