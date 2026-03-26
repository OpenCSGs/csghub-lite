BINARY_NAME := csghub-lite
RELEASE_TAG ?= $(shell git describe --tags --exact-match 2>/dev/null || true)
VERSION := $(if $(RELEASE_TAG),$(patsubst v%,%,$(RELEASE_TAG)),$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev"))
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build build-web build-all clean-dist package release install test test-cover lint clean release-snapshot

build-web:
	cd web && npm install && npm run build
	rm -rf internal/server/static/assets internal/server/static/*.html internal/server/static/*.js internal/server/static/*.css internal/server/static/*.svg
	cp -r web/dist/* internal/server/static/

build: build-web
	go build $(LDFLAGS) -o bin/$(BINARY_NAME)-$(VERSION) ./cmd/csghub-lite

build-all: build-darwin-arm64 build-darwin-amd64 build-linux-amd64 build-linux-arm64 build-windows-amd64

# Cross-platform binaries must embed the built web UI too.
build-darwin-arm64: build-web
	GOOS=darwin GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-$(VERSION)-darwin-arm64 ./cmd/csghub-lite

build-darwin-amd64: build-web
	GOOS=darwin GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-$(VERSION)-darwin-amd64 ./cmd/csghub-lite

build-linux-amd64: build-web
	GOOS=linux GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-$(VERSION)-linux-amd64 ./cmd/csghub-lite

build-linux-arm64: build-web
	GOOS=linux GOARCH=arm64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-$(VERSION)-linux-arm64 ./cmd/csghub-lite

build-windows-amd64: build-web
	GOOS=windows GOARCH=amd64 go build $(LDFLAGS) -o bin/$(BINARY_NAME)-$(VERSION)-windows-amd64.exe ./cmd/csghub-lite

clean-dist:
	@rm -rf dist
	@mkdir -p dist

package: clean-dist build-all
	@for platform in darwin-arm64 darwin-amd64 linux-amd64 linux-arm64; do \
		ARCHIVE="dist/$(BINARY_NAME)_$(VERSION)_$${platform}.tar.gz"; \
		TMPDIR=$$(mktemp -d); \
		cp bin/$(BINARY_NAME)-$(VERSION)-$${platform} $${TMPDIR}/$(BINARY_NAME); \
		cp README.md $${TMPDIR}/; \
		COPYFILE_DISABLE=1 tar czf $${ARCHIVE} --no-xattrs -C $${TMPDIR} .; \
		rm -rf $${TMPDIR}; \
		echo "Created $${ARCHIVE}"; \
	done
	@TMPDIR=$$(mktemp -d); \
	cp bin/$(BINARY_NAME)-$(VERSION)-windows-amd64.exe $${TMPDIR}/$(BINARY_NAME).exe; \
	cp README.md $${TMPDIR}/; \
	cd $${TMPDIR} && zip -q -r $(CURDIR)/dist/$(BINARY_NAME)_$(VERSION)_windows-amd64.zip *; \
	rm -rf $${TMPDIR}; \
	echo "Created dist/$(BINARY_NAME)_$(VERSION)_windows-amd64.zip"
	@./scripts/write-checksums.sh dist
	@echo "Created dist/checksums.txt"

release:
	@scripts/push.sh

install: build
	install -d $(DESTDIR)/usr/local/bin
	install -m 755 bin/$(BINARY_NAME)-$(VERSION) $(DESTDIR)/usr/local/bin/$(BINARY_NAME)

test:
	go test -race -count=1 ./...

test-cover:
	go test -race -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

lint:
	golangci-lint run ./...

release-snapshot: build-web
	goreleaser release --snapshot --clean

clean:
	rm -rf bin/ dist/ coverage.out coverage.html
