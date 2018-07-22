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

ARCH      := "`uname -s`"
LINUX     := "Linux"
MAC       := "Darwin"
GO_FILES=$(find . -iname '*.go' | grep -v /vendor/)
PKGS=$(go list ./... | grep -v /vendor/)

.PHONY: all build update test

default: build

build: config

format:
	gofmt -l -s -w .

update-ui:
	@echo "generate static files"
	@(cd cmd/static_fs && $(GOBUILD) -o ../../bin/static_fs)
	@bin/static_fs -ignore="static.go|.DS_Store" -o=static/static.go -pkg=static static

update-template-ui:
	@echo "generate UI pages"
	@$(GO) get github.com/infinitbyte/ego/cmd/ego
	@cd core/ && ego
	@cd modules/ && ego
	@cd plugins/ && ego

config: update-ui update-template-ui

fetch-depends:
	@echo "fetch dependencies"
	$(GO) get github.com/cihub/seelog
	$(GO) get github.com/PuerkitoBio/purell
	$(GO) get github.com/clarkduvall/hyperloglog
	$(GO) get github.com/PuerkitoBio/goquery
	$(GO) get github.com/jmoiron/jsonq
	$(GO) get github.com/gorilla/websocket
	$(GO) get github.com/boltdb/bolt/...
	$(GO) get github.com/alash3al/goemitter
	$(GO) get github.com/bkaradzic/go-lz4
	$(GO) get github.com/elgs/gojq
	$(GO) get github.com/kardianos/osext
	$(GO) get github.com/zeebo/sbloom
	$(GO) get github.com/asdine/storm
	$(GO) get github.com/rs/xid
	$(GO) get github.com/seiflotfy/cuckoofilter
	$(GO) get github.com/hashicorp/raft
	$(GO) get github.com/hashicorp/raft-boltdb
	$(GO) get github.com/jaytaylor/html2text
	$(GO) get github.com/asdine/storm/codec/protobuf
	$(GO) get github.com/ryanuber/go-glob
	$(GO) get github.com/gorilla/sessions
	$(GO) get github.com/mattn/go-sqlite3
	$(GO) get github.com/jinzhu/gorm
	$(GO) get github.com/stretchr/testify/assert
	$(GO) get github.com/spf13/viper
	$(GO) get -t github.com/RoaringBitmap/roaring
	$(GO) get github.com/elastic/go-ucfg
	$(GO) get github.com/jasonlvhit/gocron
	$(GO) get github.com/quipo/statsd
	$(GO) get github.com/go-sql-driver/mysql
	$(GO) get github.com/jbowles/cld2_nlpt
	$(GO) get github.com/mafredri/cdp
	$(GO) get github.com/ararog/timeago
	$(GO) get github.com/google/go-github/github
	$(GO) get golang.org/x/oauth2
	$(GO) get github.com/rs/cors

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
