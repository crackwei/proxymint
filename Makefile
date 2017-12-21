UNAME=$(shell uname)
PREFIX=github.com/BTCChina/mining-pool-proxy
GOPATH=$(shell go env GOPATH)
GOVERSION=$(shell go version)
BUILD_SHA=$(shell git rev-parse HEAD)

ifeq "$(UNAME)" "Darwin"
    BUILD_FLAGS=-ldflags="-s -X main.Build=$(BUILD_SHA)"
else
    BUILD_FLAGS=-ldflags="-X main.Build=$(BUILD_SHA)"
endif

ALL_PACKAGES=$(shell go list ./... | grep -v /vendor/)

TEST_FLAGS=-count 1

build: 
	go build

install: 
	go install

test:
# ifdef DARWIN
# 	sudo -E go test -p 1 -count 1 -v $(ALL_PACKAGES)
# else
# 	go test -p 1 $(TEST_FLAGS) $(BUILD_FLAGS) $(ALL_PACKAGES)
# endif

vendor: glide.yaml
	@glide up -v

all: vendor test install build

.PHONY: all