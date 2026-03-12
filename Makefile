BINARY_NAME := agentcom
BUILD_DIR := bin
MAIN_PKG := ./cmd/agentcom

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
BUILD_DATE := $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')
GO_VERSION := $(shell go version | awk '{print $$3}')

LDFLAGS := -ldflags "\
	-X github.com/malleus35/agentcom/internal/cli.Version=$(VERSION) \
	-X github.com/malleus35/agentcom/internal/cli.BuildDate=$(BUILD_DATE) \
	-X github.com/malleus35/agentcom/internal/cli.GoVersion=$(GO_VERSION)"

.PHONY: build test lint clean install fmt vet

build:
	@mkdir -p $(BUILD_DIR)
	CGO_ENABLED=1 go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) $(MAIN_PKG)

test:
	CGO_ENABLED=1 go test -race -count=1 ./...

lint:
	golangci-lint run ./...

clean:
	rm -rf $(BUILD_DIR)
	go clean

install:
	CGO_ENABLED=1 go install $(LDFLAGS) $(MAIN_PKG)

fmt:
	gofmt -w .

vet:
	go vet ./...

coverage:
	CGO_ENABLED=1 go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
