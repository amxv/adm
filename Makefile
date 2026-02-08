VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X main.version=$(VERSION)
BINARY  := adm
DIST    := dist

PLATFORMS := darwin/amd64 darwin/arm64 linux/amd64 linux/arm64

.PHONY: build clean release checksums ui

# Build frontend assets.
ui:
	cd ui && bun install && bun run build

# Build for the current platform (includes frontend).
build: ui
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/adm

# Cross-compile for all platforms and package as tar.gz.
release: clean ui
	@mkdir -p $(DIST)
	@for platform in $(PLATFORMS); do \
		os=$${platform%/*}; \
		arch=$${platform#*/}; \
		name=$(BINARY)_$(VERSION)_$${os}_$${arch}; \
		echo "Building $${name}..."; \
		GOOS=$${os} GOARCH=$${arch} go build -ldflags "$(LDFLAGS)" -o $(DIST)/$${name}/$(BINARY) ./cmd/adm; \
		tar -czf $(DIST)/$${name}.tar.gz -C $(DIST) $${name}; \
		rm -rf $(DIST)/$${name}; \
	done
	@$(MAKE) checksums

# Generate SHA-256 checksums for all archives.
checksums:
	@cd $(DIST) && shasum -a 256 *.tar.gz > checksums.txt
	@echo "Checksums written to $(DIST)/checksums.txt"

clean:
	rm -rf $(DIST)
	rm -f $(BINARY)
