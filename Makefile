BINARY_NAME := csghub-lite
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build build-all install test test-cover lint clean release-snapshot

build:
	go build $(LDFLAGS) -o bin/$(BINARY_NAME) ./cmd/csghub-lite

build-all: build-darwin-arm64 build-darwin-amd64 build-linux-amd64 build-linux-arm64 build-windows-amd64

build-darwin-arm64:
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-arm64 ./cmd/csghub-lite

build-darwin-amd64:
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-darwin-amd64 ./cmd/csghub-lite

build-linux-amd64:
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-amd64 ./cmd/csghub-lite

build-linux-arm64:
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-linux-arm64 ./cmd/csghub-lite

build-windows-amd64:
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-windows-amd64.exe ./cmd/csghub-lite

install: build
	install -d $(DESTDIR)/usr/local/bin
	install -m 755 bin/$(BINARY_NAME) $(DESTDIR)/usr/local/bin/$(BINARY_NAME)

test:
	go test -race -count=1 ./...

test-cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run ./...

release-snapshot:
	goreleaser release --snapshot --clean

clean:
	rm -rf bin/ dist/ coverage.out coverage.html
