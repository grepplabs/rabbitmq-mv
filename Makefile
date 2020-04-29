.DEFAULT_GOAL := build

.PHONY: clean build fmt test

BUILD_FLAGS   ?=
BINARY        ?= rabbitmq-mv
VERSION       ?= $(shell git describe --tags --always --dirty)
LDFLAGS       ?= -w -s

GOARCH        ?= amd64
GOOS          ?= linux

LOCAL_IMAGE   ?= local/$(GOOS)-$(GOARCH)/$(BINARY)
LOCAL_BIN     ?= $(BINARY)-$(GOOS)-$(GOARCH)

ROOT_DIR      := $(shell dirname $(realpath $(lastword $(MAKEFILE_LIST))))

default: build

test:
	GO111MODULE=on go test -mod=vendor -v ./...

build:
	CGO_ENABLED=0 GO111MODULE=on go build -mod=vendor -o $(BINARY) $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" .

.PHONY: os.build
os.build:
	GOOS=$(GOOS) GOARCH=$(GOARCH) CGO_ENABLED=0 GO111MODULE=on go build -mod=vendor -o $(BINARY) $(BUILD_FLAGS) -ldflags "$(LDFLAGS)" .

.PHONY: docker.build
docker.build:
	docker build --build-arg GOOS=$(GOOS) --build-arg GOARCH=$(GOARCH) -f Dockerfile -t $(LOCAL_IMAGE) .

.PHONY: docker.build
docker.copy: docker.build
	$(eval BUILDCONTAINER=$(shell sh -c "docker create $(LOCAL_IMAGE)"))
	$(shell docker cp $(BUILDCONTAINER):/rabbitmq-mv ./$(LOCAL_BIN))
	$(eval RESULT=$(shell sh -c "docker rm $(BUILDCONTAINER)"))
	$(eval RESULT=$(shell sh -c "docker rmi $(LOCAL_IMAGE)"))
	echo "Binary copied to local directory"

.PHONY: build.os
build.os: clean docker.copy

.PHONY: build.linux
build.linux: clean
	make GOOS=linux LOCAL_BIN=rabbitmq-mv docker.copy

.PHONY: build.darwin
build.darwin: clean
	make GOOS=darwin LOCAL_BIN=rabbitmq-mv docker.copy

.PHONY: build.windows
build.windows: clean
	make GOOS=windows LOCAL_BIN=rabbitmq-mv.exe docker.copy

.PHONY: build.dist
build.dist: clean
	make GOOS=linux LOCAL_BIN=rabbitmq-mv-linux docker.copy
	make GOOS=darwin LOCAL_BIN=rabbitmq-mv-darwin docker.copy
	make GOOS=windows LOCAL_BIN=rabbitmq-mv-windows docker.copy

fmt:
	go fmt ./...

clean:
	@rm -rf $(BINARY)
	@rm -rf $(BINARY)*

.PHONY: deps
deps:
	GO111MODULE=on go get ./...

.PHONY: vendor
vendor:
	GO111MODULE=on go mod vendor

.PHONY: tidy
tidy:
	GO111MODULE=on go mod tidy

