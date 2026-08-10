.PHONY: all build dev clean install help test deps

# Note: Go plugins only support Linux and macOS (Darwin)

# Colors
COLOR_RESET = \033[0m
COLOR_INFO = \033[36m
COLOR_SUCCESS = \033[32m
COLOR_WARNING = \033[33m
COLOR_ERROR = \033[31m
COLOR_BOLD = \033[1m

PLUGIN_NAME = reasoning-flag-controller
OUTPUT_DIR = build

# Platform detection
UNAME_S := $(shell uname -s)
ifeq ($(UNAME_S),Linux)
	PLUGIN_EXT = .so
	PLATFORM = linux
	HOST_OS = linux
endif
ifeq ($(UNAME_S),Darwin)
	PLUGIN_EXT = .so
	PLATFORM = darwin
	HOST_OS = darwin
endif

# Architecture detection
UNAME_M := $(shell uname -m)
ifeq ($(UNAME_M),x86_64)
	ARCH = amd64
	HOST_ARCH = amd64
endif
ifeq ($(UNAME_M),arm64)
	ARCH = arm64
	HOST_ARCH = arm64
endif
ifeq ($(UNAME_M),aarch64)
	ARCH = arm64
	HOST_ARCH = arm64
endif

GOOS ?=
GOARCH ?=

OUTPUT = $(OUTPUT_DIR)/$(PLUGIN_NAME)$(PLUGIN_EXT)

help: ## Show this help message
	@echo '$(COLOR_BOLD)Usage:$(COLOR_RESET) make [target] [GOOS=...] [GOARCH=...]'
	@echo ''
	@echo '$(COLOR_BOLD)Examples:$(COLOR_RESET)'
	@echo '  make dev                          # Build for development'
	@echo '  make build                        # Build for current platform'
	@echo '  make build GOOS=linux GOARCH=amd64'
	@echo ''
	@echo '$(COLOR_BOLD)Available targets:$(COLOR_RESET)'
	@awk 'BEGIN {FS = ":.*?## "} /^[a-zA-Z_-]+:.*?## / {printf "  $(COLOR_INFO)%-20s$(COLOR_RESET) %s\n", $$1, $$2}' $(MAKEFILE_LIST)

_clean_build_dir:
	@rm -rf $(OUTPUT_DIR)

build: _clean_build_dir ## Build the plugin (supports: make build GOOS=linux GOARCH=amd64)
	@mkdir -p $(OUTPUT_DIR)
	@TARGET_OS="$(GOOS)"; \
	TARGET_ARCH="$(GOARCH)"; \
	ACTUAL_OS=$$(uname -s | tr '[:upper:]' '[:lower:]'); \
	ACTUAL_ARCH=$$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/;s/arm64/arm64/'); \
	if [ -z "$$TARGET_OS" ]; then \
		TARGET_OS=$$ACTUAL_OS; \
	fi; \
	if [ -z "$$TARGET_ARCH" ]; then \
		TARGET_ARCH=$$ACTUAL_ARCH; \
	fi; \
	HOST_OS=$$ACTUAL_OS; \
	HOST_ARCH=$$ACTUAL_ARCH; \
	echo "$(COLOR_INFO)Host: $$HOST_OS/$$HOST_ARCH | Target: $$TARGET_OS/$$TARGET_ARCH$(COLOR_RESET)"; \
	if [ "$$TARGET_OS" = "$$HOST_OS" ] && [ "$$TARGET_ARCH" = "$$HOST_ARCH" ]; then \
		echo "$(COLOR_INFO)Building plugin for $$TARGET_OS/$$TARGET_ARCH (native build)...$(COLOR_RESET)"; \
		CGO_ENABLED=1 GOOS=$$TARGET_OS GOARCH=$$TARGET_ARCH \
			go build -buildmode=plugin -o $(OUTPUT) .; \
		echo "$(COLOR_SUCCESS)✓ Plugin built successfully: $(OUTPUT)$(COLOR_RESET)"; \
	elif [ "$$HOST_OS" = "darwin" ] && [ "$$TARGET_OS" = "darwin" ]; then \
		echo "$(COLOR_INFO)Building plugin for $$TARGET_OS/$$TARGET_ARCH (macOS cross-arch)...$(COLOR_RESET)"; \
		CGO_ENABLED=1 GOOS=$$TARGET_OS GOARCH=$$TARGET_ARCH \
			go build -buildmode=plugin -ldflags="-w -s" -trimpath -o $(OUTPUT) .; \
		echo "$(COLOR_SUCCESS)✓ Plugin built successfully: $(OUTPUT)$(COLOR_RESET)"; \
	else \
		echo "$(COLOR_WARNING)Cross-compilation detected: $$HOST_OS/$$HOST_ARCH -> $$TARGET_OS/$$TARGET_ARCH$(COLOR_RESET)"; \
		echo "$(COLOR_INFO)Using Docker for cross-compilation...$(COLOR_RESET)"; \
		$(MAKE) _build-with-docker TARGET_OS=$$TARGET_OS TARGET_ARCH=$$TARGET_ARCH; \
	fi

dev: _clean_build_dir ## Build the plugin for development (no optimization flags)
	@mkdir -p $(OUTPUT_DIR)
	@echo "$(COLOR_INFO)Building plugin for development...$(COLOR_RESET)"
	CGO_ENABLED=1 go build -buildmode=plugin -o $(OUTPUT) .
	@echo "$(COLOR_SUCCESS)✓ Plugin built successfully: $(OUTPUT)$(COLOR_RESET)"

_build-with-docker:
	@if [ "$(TARGET_OS)" = "linux" ]; then \
		echo "$(COLOR_INFO)Building for $(TARGET_OS)/$(TARGET_ARCH) in Docker...$(COLOR_RESET)"; \
		docker run --rm \
			--platform linux/$(TARGET_ARCH) \
			-v "$(PWD):/work" \
			-w /work \
			-e CGO_ENABLED=1 \
			-e GOOS=$(TARGET_OS) \
			-e GOARCH=$(TARGET_ARCH) \
			golang:1.26.5-alpine3.23 \
			sh -c "apk add --no-cache gcc musl-dev && \
				go build -buildmode=plugin -ldflags='-w -s' -trimpath -o $(OUTPUT) ."; \
		echo "$(COLOR_SUCCESS)✓ Plugin built successfully: $(OUTPUT)$(COLOR_RESET)"; \
	else \
		echo "$(COLOR_ERROR)✗ Docker cross-compilation only supports Linux targets$(COLOR_RESET)"; \
		exit 1; \
	fi

clean: _clean_build_dir ## Remove build artifacts
	@rm -rf $(OUTPUT_DIR)
	@echo "$(COLOR_SUCCESS)✓ Clean complete$(COLOR_RESET)"

install: build ## Build and install to ~/.bifrost/plugins
	@mkdir -p ~/.bifrost/plugins
	@cp $(OUTPUT) ~/.bifrost/plugins/
	@echo "$(COLOR_SUCCESS)✓ Installed to ~/.bifrost/plugins/$(PLUGIN_NAME)$(PLUGIN_EXT)$(COLOR_RESET)"

test: ## Run unit tests
	go test -v ./...

deps: ## Download and tidy dependencies
	go mod download
	go mod tidy

.DEFAULT_GOAL := help
