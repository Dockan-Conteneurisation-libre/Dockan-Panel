# Dockan Panel: Vue interface embedded in a single static Go binary.
VERSION ?= $(shell sed -n 's/^var Version = "\(.*\)"$$/\1/p' internal/config/config.go)
GOFLAGS_BUILD = -trimpath -ldflags "-s -w -X dockan-panel/internal/config.Version=$(VERSION)"
DIST = dist

.PHONY: all web build test test-ci dist clean

all: build

web:
	cd web && npm ci && npm run build

build: web
	CGO_ENABLED=0 go build $(GOFLAGS_BUILD) -o dockan-panel ./cmd/dockan-panel

test:
	go vet ./...
	go test ./...
	cd web && npm run typecheck && npm test

# Clean install of the web dependencies, then every check.
test-ci: web
	cd web && npm test
	go vet ./...
	go test ./...

# Release archives: dist/dockan-panel-linux-<arch>.tar.gz and SHA256SUMS.
dist: web
	rm -rf $(DIST) && mkdir -p $(DIST)
	for arch in amd64 arm64; do \
	  dir=$(DIST)/$$arch/dockan-panel; mkdir -p $$dir; \
	  CGO_ENABLED=0 GOOS=linux GOARCH=$$arch go build $(GOFLAGS_BUILD) -o $$dir/dockan-panel ./cmd/dockan-panel; \
	  cp README.md deploy/Dockanfile deploy/dockan.yml scripts/restore-prod-storage.sh $$dir/; \
	  tar -C $(DIST)/$$arch --owner=0 --group=0 -czf $(DIST)/dockan-panel-linux-$$arch.tar.gz dockan-panel; \
	done
	cd $(DIST) && sha256sum dockan-panel-linux-*.tar.gz > SHA256SUMS

clean:
	rm -rf $(DIST) dockan-panel
