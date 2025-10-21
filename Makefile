SHELL=/bin/bash

# APP info
APP_NAME?= framework
APP_VERSION?= 1.0.0_SNAPSHOT
APP_CONFIG ?= $(APP_NAME).yml
APP_EOLDate ?= "2023-12-31T10:10:10Z"
APP_STATIC_FOLDER ?= .public
APP_STATIC_PACKAGE ?= public
APP_UI_FOLDER ?= ui
APP_PLUGIN_FOLDER ?= plugins
APP_PLUGIN_PKG ?= $(APP_PLUGIN_FOLDER)
APP_NEED_CGO ?= 0

# Get release version from environment
ifneq '$(VERSION)' ''
   APP_VERSION := $(VERSION)
endif

ifneq '$(EOL)' ''
   APP_EOLDate := $(EOL)
endif

# Ensure GOPATH is set before running build process.
ifeq "$(GOPATH)" ""
   GOPATH := $(HOME)/go
  #$(error Please set the environment variable GOPATH before running `make`)
endif

# COMMIT_ID=$(shell git log -1 --pretty=format:"%h, %ad, %an, %s")
COMMIT_ID=$(shell git rev-parse HEAD)
NOW=$(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
BUILD_NUMBER ?= 001

## this will grow the binary size
# GOBUILD_FLAGS?=-ldflags "-X infini.sh/framework/core/env.version=$(APP_VERSION)  -X infini.sh/framework/core/env.buildDate=$(NOW)   -X infini.sh/framework/core/env.commit=$(COMMIT_ID) -X infini.sh/framework/core/env.eolDate=$(APP_EOLDate)  -X infini.sh/framework/core/env.buildNumber=$(BUILD_NUMBER)"

PATH := $(PATH):$(GOPATH)/bin
VFS_PATH := $(HOME)/vfs

# Go environment
CURDIR := $(shell pwd)
OLDGOPATH:= $(HOME)/go

CMD_DIR := $(CURDIR)/cmd
OUTPUT_DIR := $(CURDIR)/bin


# INFINI framework
INFINI_BASE_FOLDER := $(OLDGOPATH)/src/infini.sh
FRAMEWORK_FOLDER ?= $(INFINI_BASE_FOLDER)/framework
FRAMEWORK_REPO ?= https://github.com/infinilabs/framework.git
ifeq "$(FRAMEWORK_BRANCH)" ""
    FRAMEWORK_BRANCH := main
endif

FRAMEWORK_VENDOR_FOLDER ?= $(FRAMEWORK_FOLDER)/../vendor/
FRAMEWORK_VENDOR_REPO ?=  https://github.com/infinilabs/framework-vendor.git
ifeq "$(FRAMEWORK_VENDOR_BRANCH)" ""
    FRAMEWORK_VENDOR_BRANCH := main
endif

ifneq "$(DEV)" ""
    FRAMEWORK_DEVEL_BUILD := -tags dev
endif

# Adjust the vendor priority
PREFER_MANAGED_VENDOR ?= true
NEWGOPATH:= $(FRAMEWORK_VENDOR_FOLDER):$(GOPATH)
ifneq "$(PREFER_MANAGED_VENDOR)" "true"
    NEWGOPATH:= $(GOPATH):$(FRAMEWORK_VENDOR_FOLDER)
endif

GO        := go
GOMODULE ?= true
ifneq "$(GOMODULE)" "true"
    GO        := GO15VENDOREXPERIMENT="1" GO111MODULE=off go
endif
GOBUILD  := GOPATH=$(NEWGOPATH) CGO_ENABLED=$(APP_NEED_CGO) GRPC_GO_REQUIRE_HANDSHAKE=off $(GO) build -a $(FRAMEWORK_DEVEL_BUILD) -gcflags=all="-l -B"  -ldflags '-static' -ldflags='-s -w' -gcflags "-m"  --work $(GOBUILD_FLAGS)
GOBUILDDBG  := GOPATH=$(NEWGOPATH) CGO_ENABLED=$(APP_NEED_CGO) GRPC_GO_REQUIRE_HANDSHAKE=off $(GO) build -a $(FRAMEWORK_DEVEL_BUILD) -ldflags -v -gcflags "all=-N -l" --work $(GOBUILD_FLAGS)
GOBUILDNCGO  := GOPATH=$(NEWGOPATH) CGO_ENABLED=1  $(GO) build -ldflags -s $(GOBUILD_FLAGS)
GOTEST   := GOPATH=$(NEWGOPATH) CGO_ENABLED=$(APP_NEED_CGO) $(GO) test -ldflags -s

ARCH      := "`uname -s`"
LINUX     := "Linux"
MAC       := "Darwin"
GO_FILES=$(find . -iname '*.go' | grep -v /vendor/)
PKGS=$(go list ./... | grep -v /vendor/)

FRAMEWORK_OFFLINE_BUILD := ""
ifneq "$(OFFLINE_BUILD)" ""
    FRAMEWORK_OFFLINE_BUILD := $(OFFLINE_BUILD)
endif


.PHONY: all build update test clean

default: build-race

env:
	@echo OLDGOPATH：$(OLDGOPATH)
	@echo GOPATH：$(GOPATH)
	@echo NEWGOPATH：$(NEWGOPATH)
	@echo INFINI_BASE_FOLDER：$(INFINI_BASE_FOLDER)
	@echo FRAMEWORK_FOLDER：$(FRAMEWORK_FOLDER)
	@echo FRAMEWORK_REPO：$(FRAMEWORK_REPO)
	@echo FRAMEWORK_VENDOR_FOLDER：$(FRAMEWORK_VENDOR_FOLDER)
	@echo FRAMEWORK_VENDOR_REPO：$(FRAMEWORK_VENDOR_REPO)

build: config
	$(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)
	@$(MAKE) restore-generated-file

build-dev: config
	$(GOBUILD) -tags dev -o $(OUTPUT_DIR)/$(APP_NAME)-dev
	@$(MAKE) restore-generated-file

build-debug: config
	$(GOBUILDDBG) -tags dev -o $(OUTPUT_DIR)/$(APP_NAME)-debug
	@$(MAKE) restore-generated-file

build-linux-amd64-debug: config
	GOOS=linux GOARCH=amd64 $(GOBUILDDBG) -tags dev -o $(OUTPUT_DIR)/$(APP_NAME)-linux-amd64-debug
	@$(MAKE) restore-generated-file

build-linux-arm64-debug: config
	GOOS=linux GOARCH=arm64 $(GOBUILDDBG) -tags dev -o $(OUTPUT_DIR)/$(APP_NAME)-linux-arm64-debug
	@$(MAKE) restore-generated-file

build-linux-amd64-dev: config
	GOOS=linux GOARCH=amd64 $(GOBUILD) -tags dev -o $(OUTPUT_DIR)/$(APP_NAME)-linux-amd64-dev
	@$(MAKE) restore-generated-file

build-linux-arm64-dev: config
	GOOS=linux GOARCH=arm64 $(GOBUILD) -tags dev -o $(OUTPUT_DIR)/$(APP_NAME)-linux-arm64-dev
	@$(MAKE) restore-generated-file

build-cmd:
	for f in $(shell ls ${CMD_DIR}); do (cd $(CMD_DIR)/$${f} && $(GOBUILD) -o $(OUTPUT_DIR)/$${f}); done
	$(MAKE) restore-generated-file

cross-build-cmd: config
	@for f in $(shell ls ${CMD_DIR}); do (cd $(CMD_DIR)/$${f} && GOOS=windows  GOARCH=amd64 $(GOBUILD) -o $(OUTPUT_DIR)/$${f}-windows-amd64.exe); done
	@for f in $(shell ls ${CMD_DIR}); do (cd $(CMD_DIR)/$${f} && GOOS=linux  GOARCH=amd64 $(GOBUILD) -o $(OUTPUT_DIR)/$${f}-linux-amd64); done
	@$(MAKE) restore-generated-file

update-plugins:
	@if [ ! -e $(OLDGOPATH)/src/infini.sh/framework/bin/plugin-discovery ]; then ( cd $(OLDGOPATH)/src/infini.sh/framework/ && make build-cmd ) fi
	@$(foreach var,$(APP_PLUGIN_FOLDER),\
        ( $(OLDGOPATH)/src/infini.sh/framework/bin/plugin-discovery -dir $(var) -pkg $(var) -import_prefix infini.sh/$(APP_NAME) -out $(var)/generated_plugins.go); \
    )

# used to build the binary for gdb debugging
build-race: clean config update-vfs
	$(GOBUILDNCGO) -tags dev -gcflags "-m -N -l" -race -o $(OUTPUT_DIR)/$(APP_NAME)
	@$(MAKE) restore-generated-file

tar: build
	cd $(OUTPUT_DIR) && tar cfz $(OUTPUT_DIR)/$(APP_NAME).tar.gz $(APP_NAME) $(APP_CONFIG)

cross-build: clean config update-vfs
	$(GO) test
	GOOS=windows GOARCH=amd64 $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-windows-amd64.exe
	GOOS=darwin  GOARCH=amd64 $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-mac-amd64
	GOOS=linux  GOARCH=amd64 $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-linux-amd64
	@$(MAKE) restore-generated-file

build-win-amd64: config
	CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ GOOS=windows GOARCH=amd64     $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-windows-amd64.exe
	@$(MAKE) restore-generated-file

build-win-386: config
	CC=i686-w64-mingw32-gcc   CXX=i686-w64-mingw32-g++ GOOS=windows GOARCH=386         $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-windows-386.exe
	@$(MAKE) restore-generated-file

build-linux-amd64: config
	GOOS=linux  GOARCH=amd64  $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-linux-amd64
	@$(MAKE) restore-generated-file

build-linux-386: config
	GOOS=linux  GOARCH=386    $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-linux-386
	@$(MAKE) restore-generated-file

build-linux-mips64: config
	GOOS=linux  GOARCH=mips64    $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-linux-mips64
	@$(MAKE) restore-generated-file

build-linux-mips64le: config
	GOOS=linux  GOARCH=mips64le    $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-linux-mips64le
	@$(MAKE) restore-generated-file

build-linux-armv6: config
	GOOS=linux  GOARCH=arm   GOARM=6    $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-linux-armv6
	@$(MAKE) restore-generated-file

build-linux-armv7: config
	GOOS=linux  GOARCH=arm   GOARM=7    $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-linux-armv7
	@$(MAKE) restore-generated-file

build-linux-arm64: config
	GOOS=linux  GOARCH=arm64    $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-linux-arm64
	@$(MAKE) restore-generated-file

build-linux-loong64: config
	GOOS=linux  GOARCH=loong64    $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-linux-loong64
	@$(MAKE) restore-generated-file

build-linux-riscv64: config
	GOOS=linux  GOARCH=riscv64    $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-linux-riscv64
	@$(MAKE) restore-generated-file

build-darwin-amd64: config
	GOOS=darwin  GOARCH=amd64     $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-mac-amd64
	@$(MAKE) restore-generated-file

build-darwin-arm64: config
	GOOS=darwin  GOARCH=arm64    $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-mac-arm64
	@$(MAKE) restore-generated-file

build-win: config
	CC=x86_64-w64-mingw32-gcc CXX=x86_64-w64-mingw32-g++ GOOS=windows GOARCH=amd64     $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-windows-amd64.exe
	CC=i686-w64-mingw32-gcc   CXX=i686-w64-mingw32-g++ GOOS=windows GOARCH=386         $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-windows-386.exe
	@$(MAKE) restore-generated-file

build-linux: config
	GOOS=linux  GOARCH=amd64  $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-linux-amd64
	GOOS=linux  GOARCH=386    $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-linux-386
	GOOS=linux  GOARCH=mips    $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-linux-mips
	GOOS=linux  GOARCH=mipsle    $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-linux-mipsle
	GOOS=linux  GOARCH=mips64    $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-linux-mips64
	GOOS=linux  GOARCH=mips64le    $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-linux-mips64le
	GOOS=linux  GOARCH=loong64    $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-linux-loong64
	GOOS=linux  GOARCH=riscv64    $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-linux-riscv64
	@$(MAKE) restore-generated-file

build-arm: config
	GOOS=linux  GOARCH=arm64    $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-linux-arm64
	GOOS=linux  GOARCH=arm   GOARM=5    $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-linux-armv5
	GOOS=linux  GOARCH=arm   GOARM=6    $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-linux-armv6
	GOOS=linux  GOARCH=arm   GOARM=7    $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-linux-armv7
	@$(MAKE) restore-generated-file

build-darwin: config
	GOOS=darwin  GOARCH=amd64     $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-mac-amd64
	GOOS=darwin  GOARCH=arm64    $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-mac-arm64
	@$(MAKE) restore-generated-file

build-bsd: config
	GOOS=freebsd  GOARCH=amd64    $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-freebsd-amd64
	GOOS=freebsd  GOARCH=386      $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-freebsd-386
	GOOS=netbsd  GOARCH=amd64     $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-netbsd-amd64
	GOOS=netbsd  GOARCH=386       $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-netbsd-386
	GOOS=openbsd  GOARCH=amd64    $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-openbsd-amd64
	GOOS=openbsd  GOARCH=386      $(GOBUILD) -o $(OUTPUT_DIR)/$(APP_NAME)-openbsd-386
	@$(MAKE) restore-generated-file

all: clean config update-vfs cross-build restore-generated-file

all-platform: clean config update-vfs cross-build-all-platform restore-generated-file

cross-build-all-platform: clean config build-bsd build-linux build-darwin build-win  restore-generated-file

format:
	@echo "formatting code"
	GOPATH=$(NEWGOPATH) $(GO) fmt $$(GOPATH=$(NEWGOPATH) $(GO) list ./...)

test: config
	$(GOTEST) -v $(GOFLAGS) -timeout 30m ./...
	@$(MAKE) restore-generated-file

cov:
	go test -race -covermode=atomic -coverprofile=coverage.out ./...

tidy:
	go mod tidy

lint: config
	@if ! command -v golangci-lint >/dev/null 2>&1; then \
		echo "Installing golangci-lint..."; \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/HEAD/install.sh | sh -s -- -b $(shell go env GOPATH)/bin v2.1.6; \
	fi
	golangci-lint run
	@$(MAKE) restore-generated-file

clean_data:
	rm -rif dist
	rm -rif data
	rm -rif log

clean: clean_data
	rm -rif $(OUTPUT_DIR)
	mkdir $(OUTPUT_DIR)

init:
	@echo building $(APP_NAME) $(APP_VERSION) $(FRAMEWORK_BRANCH)
	@echo $(CURDIR)
	@mkdir -p $(INFINI_BASE_FOLDER)
	@echo "framework path: " $(FRAMEWORK_FOLDER)
	@if [ ! -d $(FRAMEWORK_FOLDER) ]; then echo "framework does not exist";(cd $(INFINI_BASE_FOLDER) && git clone -b $(FRAMEWORK_BRANCH) $(FRAMEWORK_REPO) framework ) fi
	@if [ ! -d $(FRAMEWORK_VENDOR_FOLDER) ]; then echo "framework vendor does not exist";(cd $(INFINI_BASE_FOLDER) && git clone -b $(FRAMEWORK_VENDOR_BRANCH) $(FRAMEWORK_VENDOR_REPO) $(FRAMEWORK_VENDOR_FOLDER)) fi
	@if [ "" == $(FRAMEWORK_OFFLINE_BUILD) ]; then (cd $(FRAMEWORK_FOLDER) && git checkout $(FRAMEWORK_BRANCH) && git pull origin $(FRAMEWORK_BRANCH)); fi;
	@if [ "" == $(FRAMEWORK_OFFLINE_BUILD) ]; then (cd $(FRAMEWORK_VENDOR_FOLDER) && git checkout $(FRAMEWORK_VENDOR_BRANCH) && git pull origin $(FRAMEWORK_VENDOR_BRANCH)); fi;
	@# Extract the latest commit hash from the framework repository
	@(cd $(FRAMEWORK_FOLDER) && git rev-parse HEAD > $(FRAMEWORK_FOLDER)/.latest_commit_hash.txt)
	@(cd $(FRAMEWORK_VENDOR_FOLDER) && git rev-parse HEAD > $(FRAMEWORK_VENDOR_FOLDER)/.latest_commit_hash.txt)
	@echo "Framework commit hash updated: " && cat $(FRAMEWORK_FOLDER)/.latest_commit_hash.txt
	@echo "Framework vendor commit hash updated: " && cat $(FRAMEWORK_VENDOR_FOLDER)/.latest_commit_hash.txt

update-generated-framework-info:
	@echo "generating framework info"
	@if [ ! -d $(FRAMEWORK_FOLDER) ]; then echo "framework does not exist"; (make init); fi
	@# Generate the framework info file
	@(cd $(FRAMEWORK_FOLDER) && \
	 LATEST_COMMIT_LOG=$$(cat .latest_commit_hash.txt) && \
	 VENDOR_COMMIT_LOG=$$(cat $(FRAMEWORK_VENDOR_FOLDER)/.latest_commit_hash.txt) && \
	 echo -e "package config\n\nconst LastFrameworkCommitLog = \"$$LATEST_COMMIT_LOG\"\nconst LastFrameworkVendorCommitLog = \"$$VENDOR_COMMIT_LOG\"" > config/generated_framework-info.go)

update-generated-file: update-generated-framework-info
	@echo "generating application info"
	@if [ ! -d config ]; then echo "config does not exist";(mkdir config) fi
	@echo -e "package config\n\nconst LastCommitLog = \"$(COMMIT_ID)\"" > config/generated.go
	@echo -e "\nconst BuildDate = \"$(NOW)\"" >> config/generated.go
	@echo -e "\nconst EOLDate  = \"$(APP_EOLDate)\"" >> config/generated.go
	@echo -e "\nconst Version  = \"$(APP_VERSION)\"" >> config/generated.go
	@echo -e "\nconst BuildNumber  = \"$(BUILD_NUMBER)\"" >> config/generated.go

restore-generated-framework-info:
	@echo "restore framework info"
	@( cd $(FRAMEWORK_FOLDER) && echo -e "package config\n\nconst LastFrameworkCommitLog = \"N/A\"" > config/generated_framework-info.go)
	@( cd $(FRAMEWORK_FOLDER) && echo -e "\nconst LastFrameworkVendorCommitLog = \"N/A\"" >> config/generated_framework-info.go )

restore-generated-file: restore-generated-framework-info
	@echo "restore application info"
	@echo -e "package config\n\nconst LastCommitLog = \"N/A\"" > config/generated.go
	@echo -e "\nconst BuildDate = \"N/A\"" >> config/generated.go
	@echo -e "\nconst EOLDate = \"N/A\"" >> config/generated.go
	@echo -e "\nconst Version = \"0.0.1-SNAPSHOT\"" >> config/generated.go
	@echo -e "\nconst BuildNumber = \"001\"" >> config/generated.go

update-vfs:
	@if [ ! -e $(VFS_PATH) ]; then (cd $(FRAMEWORK_FOLDER) && OFFLINE_BUILD=true make build-cmd && cp bin/vfs $(VFS_PATH)) fi
	@if [ -d $(APP_STATIC_FOLDER) ]; then  echo "generate static files";(cd $(APP_STATIC_FOLDER) && $(VFS_PATH) -ignore="static.go|.DS_Store" -o static.go -pkg $(APP_STATIC_PACKAGE) . ) fi

config: init format update-vfs update-generated-file update-plugins
	@echo "update configs"
	@# $(GO) env
	@mkdir -p $(OUTPUT_DIR)
	@cp $(APP_CONFIG) $(OUTPUT_DIR)
	(cd ../framework/  && make update-plugins) || true # build plugins in framework

update-license-header:
	licensure --in-place -p

dist: cross-build package

dist-major-platform: all package

dist-all-platform: all-platform package-all-platform

package:
	@echo "Packaging"
	cd $(OUTPUT_DIR) && tar cfz $(OUTPUT_DIR)/mac-amd64.tar.gz mac-amd64  $(APP_CONFIG)
	cd $(OUTPUT_DIR) && tar cfz $(OUTPUT_DIR)/linux-amd64.tar.gz linux-amd64  $(APP_CONFIG)
	cd $(OUTPUT_DIR) && tar cfz $(OUTPUT_DIR)/windows-amd64.tar.gz windows-amd64  $(APP_CONFIG)

package-all-platform: package-darwin-platform package-linux-platform package-windows-platform
	@echo "Packaging all"
	cd $(OUTPUT_DIR) && tar cfz $(OUTPUT_DIR)/freebsd-amd64.tar.gz     $(APP_NAME)-freebsd-amd64  $(APP_CONFIG)
	cd $(OUTPUT_DIR) && tar cfz $(OUTPUT_DIR)/freebsd-386.tar.gz     $(APP_NAME)-freebsd-386  $(APP_CONFIG)
	cd $(OUTPUT_DIR) && tar cfz $(OUTPUT_DIR)/netbsd-amd64.tar.gz      $(APP_NAME)-netbsd-amd64  $(APP_CONFIG)
	cd $(OUTPUT_DIR) && tar cfz $(OUTPUT_DIR)/netbsd-386.tar.gz      $(APP_NAME)-netbsd-386  $(APP_CONFIG)
	cd $(OUTPUT_DIR) && tar cfz $(OUTPUT_DIR)/openbsd-amd64.tar.gz     $(APP_NAME)-openbsd-amd64  $(APP_CONFIG)
	cd $(OUTPUT_DIR) && tar cfz $(OUTPUT_DIR)/openbsd-386.tar.gz     $(APP_NAME)-openbsd-386  $(APP_CONFIG)

package-darwin-platform:
	@echo "Packaging Darwin"
	cd $(OUTPUT_DIR) && zip -r $(OUTPUT_DIR)/mac-amd64.zip      $(APP_NAME)-mac-amd64 $(APP_CONFIG)
# 	cd $(OUTPUT_DIR) && zip -r $(OUTPUT_DIR)/mac-386.zip      $(APP_NAME)-mac-386 $(APP_CONFIG)
	cd $(OUTPUT_DIR) && zip -r $(OUTPUT_DIR)/mac-arm64.zip      $(APP_NAME)-mac-arm64 $(APP_CONFIG)

package-linux-platform:
	@echo "Packaging Linux"
	cd $(OUTPUT_DIR) && tar cfz $(OUTPUT_DIR)/linux-amd64.tar.gz     $(APP_NAME)-linux-amd64 $(APP_CONFIG)
	cd $(OUTPUT_DIR) && tar cfz $(OUTPUT_DIR)/linux-386.tar.gz     $(APP_NAME)-linux-386 $(APP_CONFIG)
	cd $(OUTPUT_DIR) && tar cfz $(OUTPUT_DIR)/linux-mips.tar.gz     $(APP_NAME)-linux-mips $(APP_CONFIG)
	cd $(OUTPUT_DIR) && tar cfz $(OUTPUT_DIR)/linux-mipsle.tar.gz     $(APP_NAME)-linux-mipsle $(APP_CONFIG)
	cd $(OUTPUT_DIR) && tar cfz $(OUTPUT_DIR)/linux-mips64.tar.gz     $(APP_NAME)-linux-mips64 $(APP_CONFIG)
	cd $(OUTPUT_DIR) && tar cfz $(OUTPUT_DIR)/linux-mips64le.tar.gz     $(APP_NAME)-linux-mips64le $(APP_CONFIG)
	cd $(OUTPUT_DIR) && tar cfz $(OUTPUT_DIR)/linux-loong64.tar.gz     $(APP_NAME)-linux-loong64 $(APP_CONFIG)
	cd $(OUTPUT_DIR) && tar cfz $(OUTPUT_DIR)/linux-riscv64.tar.gz     $(APP_NAME)-linux-riscv64 $(APP_CONFIG)

package-linux-arm-platform:
	@echo "Packaging Linux (ARM)"
	cd $(OUTPUT_DIR) && tar cfz $(OUTPUT_DIR)/linux-arm64.tar.gz       $(APP_NAME)-linux-arm64   $(APP_CONFIG)
	cd $(OUTPUT_DIR) && tar cfz $(OUTPUT_DIR)/linux-armv5.tar.gz       $(APP_NAME)-linux-armv5   $(APP_CONFIG)
	cd $(OUTPUT_DIR) && tar cfz $(OUTPUT_DIR)/linux-armv6.tar.gz       $(APP_NAME)-linux-armv6   $(APP_CONFIG)
	cd $(OUTPUT_DIR) && tar cfz $(OUTPUT_DIR)/linux-armv7.tar.gz       $(APP_NAME)-linux-armv7   $(APP_CONFIG)

package-windows-platform:
	@echo "Packaging Windows"
	cd $(OUTPUT_DIR) && zip -r $(OUTPUT_DIR)/windows-amd64.zip   $(APP_NAME)-windows-amd64.exe $(APP_CONFIG)
	cd $(OUTPUT_DIR) && zip -r $(OUTPUT_DIR)/windows-386.zip   $(APP_NAME)-windows-386.exe $(APP_CONFIG)