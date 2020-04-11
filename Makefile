SHELL=/bin/bash

# Default version
FRAMEWORK_VERSION := 0.1.0_SNAPSHOT

# Get release version from environment
ifneq "$(VERSION)" ""
   FRAMEWORK_VERSION := $(VERSION)
endif

# Ensure GOPATH is set before running build process.
ifeq "$(GOPATH)" ""
  $(error Please set the environment variable GOPATH before running `make`)
endif

# Go environment
CURDIR := $(shell pwd)
OLDGOPATH:= $(GOPATH)
NEWGOPATH:= $(CURDIR):$(CURDIR)/vendor:$(GOPATH)

GO        := GO15VENDOREXPERIMENT="1" go
GOBUILD  := GOPATH=$(NEWGOPATH) CGO_ENABLED=1  $(GO) build -ldflags -s
GOTEST   := GOPATH=$(NEWGOPATH) CGO_ENABLED=1  $(GO) test -ldflags -s

GO111MODULE=off

ARCH      := "`uname -s`"
LINUX     := "Linux"
MAC       := "Darwin"
GO_FILES=$(find . -iname '*.go' | grep -v /vendor/)
PKGS=$(go list ./... | grep -v /vendor/)

FRAMEWORK_VENDOR_FOLDER := $(CURDIR)/vendor/
FRAMEWORK_VENDOR_BRANCH := master

FRAMEWORK_OFFLINE_BUILD := ""
ifneq "$(OFFLINE_BUILD)" ""
   FRAMEWORK_OFFLINE_BUILD := $(OFFLINE_BUILD)
endif

.PHONY: all build update test

default: build

build: config

init:
	@echo building FRAMEWORK $(FRAMEWORK_VERSION)
	@if [ ! -d $(FRAMEWORK_VENDOR_FOLDER) ]; then echo "framework vendor does not exist";(git clone  -b $(FRAMEWORK_VENDOR_BRANCH) https://github.com/infinitbyte/framework-vendor.git vendor) fi
	@if [ "" == $(FRAMEWORK_OFFLINE_BUILD) ]; then (cd vendor && git pull origin $(FRAMEWORK_VENDOR_BRANCH)); fi;


format:
	go fmt $$(go list ./... | grep -v /vendor/)

update-ui:
	@echo "generate static files"
	@(cd cmd/vfs && $(GOBUILD) -o ../../bin/vfs)
	@bin/vfs -ignore="static.go|.DS_Store" -o=static/static.go -pkg=static static

update-template-ui:
	@echo "generate UI pages"
	@$(GO) get github.com/infinitbyte/ego/cmd/ego
	@cd core/ && ego
	@cd modules/ && ego
	@cd plugins/ && ego

config: update-ui update-template-ui

fetch-depends:
	@echo "fetch dependencies"
	$(GO) get -u github.com/cihub/seelog
	$(GO) get -u github.com/PuerkitoBio/purell
	$(GO) get -u github.com/clarkduvall/hyperloglog
	$(GO) get -u github.com/PuerkitoBio/goquery
	$(GO) get -u github.com/jmoiron/jsonq
	$(GO) get -u github.com/gorilla/websocket
	$(GO) get -u github.com/boltdb/bolt/...
	$(GO) get -u github.com/alash3al/goemitter
	$(GO) get -u github.com/bkaradzic/go-lz4
	$(GO) get -u github.com/elgs/gojq
	$(GO) get -u github.com/kardianos/osext
	$(GO) get -u github.com/zeebo/sbloom
	$(GO) get -u github.com/asdine/storm
	$(GO) get -u github.com/rs/xid
	$(GO) get -u github.com/seiflotfy/cuckoofilter
	$(GO) get -u github.com/hashicorp/raft
	$(GO) get -u github.com/hashicorp/raft-boltdb
	$(GO) get -u github.com/jaytaylor/html2text
	$(GO) get -u github.com/asdine/storm/codec/protobuf
	$(GO) get -u github.com/ryanuber/go-glob
	$(GO) get -u github.com/gorilla/sessions
	$(GO) get -u github.com/mattn/go-sqlite3
	$(GO) get -u github.com/jinzhu/gorm
	$(GO) get -u github.com/stretchr/testify/assert
	$(GO) get -u github.com/spf13/viper
	$(GO) get -u github.com/RoaringBitmap/roaring
	$(GO) get -u github.com/elastic/go-ucfg
	$(GO) get -u github.com/jasonlvhit/gocron
	$(GO) get -u github.com/quipo/statsd
	$(GO) get -u github.com/go-sql-driver/mysql
	$(GO) get -u github.com/jbowles/cld2_nlpt
	$(GO) get -u github.com/mafredri/cdp
	$(GO) get -u github.com/ararog/timeago
	$(GO) get -u github.com/google/go-github/github
	$(GO) get -u golang.org/x/oauth2
	$(GO) get -u github.com/rs/cors
	$(GO) get -u google.golang.org/grpc
	$(GO) get -u golang.org/x/net/http2

test:
	go get -u github.com/kardianos/govendor
	go get github.com/stretchr/testify/assert
	govendor test +local
	#$(GO) test -timeout 60s ./... --ignore ./vendor
	#GORACE="halt_on_error=1" go test ./... -race -timeout 120s  --ignore ./vendor
	#go test -bench=. -benchmem

check:
	$(GO)  get github.com/golang/lint/golint
	$(GO)  get honnef.co/go/tools/cmd/megacheck
	test -z $(gofmt -s -l $GO_FILES)    # Fail if a .go file hasn't been formatted with gofmt
	$(GO) test -v -race $(PKGS)            # Run all the tests with the race detector enabled
	$(GO) vet $(PKGS)                      # go vet is the official Go static analyzer
	@echo "go tool vet"
	go tool vet main.go
	go tool vet core
	go tool vet modules
	megacheck $(PKGS)                      # "go vet on steroids" + linter
	golint -set_exit_status $(PKGS)    # one last linter

errcheck:
	go get github.com/kisielk/errcheck
	errcheck -blank $(PKGS)

cover:
	go get github.com/mattn/goveralls
	go test -v -cover -race -coverprofile=data/coverage.out
	goveralls -coverprofile=data/coverage.out -service=travis-ci -repotoken=$COVERALLS_TOKEN

cyclo:
	go get -u github.com/fzipp/gocyclo
	gocyclo -top 10 -over 12 $$(ls -d */ | grep -v vendor)

benchmarks:
	go test github.com/infinitbyte/gopa/core/util -benchtime=1s -bench ^Benchmark -run ^$
	go test github.com/infinitbyte/gopa//modules/crawler/pipe -benchtime=1s -bench  ^Benchmark -run ^$

update_proto:
	(cd core/cluster/pb && protoc --go_out=plugins=grpc:. *.proto)
