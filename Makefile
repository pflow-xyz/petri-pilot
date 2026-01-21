.PHONY: build test clean validate-all codegen-all build-examples regenerate-imports help
.PHONY: $(EXAMPLE_TARGETS) $(CODEGEN_TARGETS)
.PHONY: $(addprefix run-,$(EXAMPLE_NAMES))

# Default target shows help
.DEFAULT_GOAL := help

# Binary name
BINARY := petri-pilot

# Example files (exclude app specs and models with known codegen issues)
EXAMPLES := $(filter-out examples/task-manager-app.json examples/order-system.json examples/token-ledger.json,$(wildcard examples/*.json))
EXAMPLE_NAMES := $(basename $(notdir $(EXAMPLES)))

# Generate target names
EXAMPLE_TARGETS := $(addprefix validate-,$(EXAMPLE_NAMES))
CODEGEN_TARGETS := $(addprefix codegen-,$(EXAMPLE_NAMES))

# Default output directory for codegen
OUTPUT_DIR ?= ./generated

# Build the binary
build:
	go build -o $(BINARY) ./cmd/petri-pilot

# Run all tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -f $(BINARY)
	rm -rf $(OUTPUT_DIR)/*/

# Validate all examples
validate-all: $(EXAMPLE_TARGETS)

# Generate code for all examples as submodules
codegen-all: $(CODEGEN_TARGETS) regenerate-imports

# Generate and build the unified binary (verifies all services compile)
build-examples: codegen-all
	@echo "=== Building unified petri-pilot binary ==="
	go build -o $(BINARY) ./cmd/petri-pilot
	@echo "=== Verifying registered services ==="
	./$(BINARY) serve
	@echo "Build successful!"

# Regenerate generated/imports.go with all service imports
regenerate-imports:
	@echo "=== Regenerating generated/imports.go ==="
	@echo '// Package generated provides auto-discovery of all generated Petri-pilot services.' > $(OUTPUT_DIR)/imports.go
	@echo '// This file imports all service packages to trigger their init() functions,' >> $(OUTPUT_DIR)/imports.go
	@echo '// which register them with the serve package.' >> $(OUTPUT_DIR)/imports.go
	@echo '//' >> $(OUTPUT_DIR)/imports.go
	@echo '// This file is auto-generated. Do not edit manually.' >> $(OUTPUT_DIR)/imports.go
	@echo '// Regenerate with: make codegen-all' >> $(OUTPUT_DIR)/imports.go
	@echo 'package generated' >> $(OUTPUT_DIR)/imports.go
	@echo '' >> $(OUTPUT_DIR)/imports.go
	@echo 'import (' >> $(OUTPUT_DIR)/imports.go
	@for name in $(EXAMPLE_NAMES); do \
		pkgname=$$(echo $$name | tr -d '-'); \
		echo "	_ \"github.com/pflow-xyz/petri-pilot/generated/$$pkgname\"" >> $(OUTPUT_DIR)/imports.go; \
	done
	@echo ')' >> $(OUTPUT_DIR)/imports.go

# Individual validate targets
# Use -tags noserve to avoid requiring generated packages during validation
validate-%: examples/%.json
	@echo "=== Validating $< ==="
	go run -tags noserve ./cmd/petri-pilot/... validate $<

# Individual codegen targets (as submodules, no go.mod)
# Use -tags noserve to avoid circular dependency on generated packages
codegen-%: examples/%.json
	@echo "=== Generating code from $< ==="
	@# Extract sanitized package name (remove hyphens)
	@pkgname=$$(echo $* | tr -d '-'); \
	rm -f $(OUTPUT_DIR)/$$pkgname/main.go $(OUTPUT_DIR)/$$pkgname/go.mod $(OUTPUT_DIR)/$$pkgname/go.sum 2>/dev/null || true; \
	mkdir -p $(OUTPUT_DIR)/$$pkgname && \
	go run -tags noserve ./cmd/petri-pilot/... codegen -submodule -pkg $$pkgname -o $(OUTPUT_DIR)/$$pkgname --frontend $<

# Run individual examples using unified CLI
run-%: build
	@echo "=== Running $* ==="
	@# Build frontend if it exists
	@pkgname=$$(echo $* | tr -d '-'); \
	if [ -d $(OUTPUT_DIR)/$$pkgname/frontend ]; then \
		echo "Building frontend..." && \
		cd $(OUTPUT_DIR)/$$pkgname/frontend && npm install && npm run build && cd -; \
	fi
	@# Determine service name from the model JSON
	@pkgname=$$(echo $* | tr -d '-'); \
	model_name=$$(grep -o '"name": *"[^"]*"' examples/$*.json 2>/dev/null | head -1 | sed 's/"name": *"//;s/"//') && \
	if [ -z "$$model_name" ]; then model_name=$$pkgname; fi && \
	echo "Starting $$model_name..." && \
	./$(BINARY) serve "$$model_name"

# Run MCP server (doesn't need generated packages)
mcp:
	go run -tags noserve ./cmd/petri-pilot/... mcp

# Generate and auto-validate a model from requirements
generate:
	@if [ -z "$(REQ)" ]; then echo "Usage: make generate REQ='your requirements'"; exit 1; fi
	go run -tags noserve ./cmd/petri-pilot/... generate -auto "$(REQ)"

# E2E tests
e2e: build-examples
	@echo "=== Running E2E tests ==="
	@cd e2e && npm install && npm test

e2e-headed: build-examples
	@echo "=== Running E2E tests (headed) ==="
	@cd e2e && npm install && npm run test:headed

e2e-debug: build-examples
	@echo "=== Running E2E tests (debug mode) ==="
	@cd e2e && npm install && npm run test:debug

# Help target
help:
	@echo "Petri Pilot Makefile"
	@echo ""
	@echo "Build targets:"
	@echo "  build          Build the petri-pilot binary"
	@echo "  test           Run all tests"
	@echo "  clean          Remove build artifacts"
	@echo ""
	@echo "Validation targets:"
	@echo "  validate-all   Validate all examples"
	@for name in $(EXAMPLE_NAMES); do \
		echo "  validate-$$name"; \
	done
	@echo ""
	@echo "Codegen targets (output to $(OUTPUT_DIR)/<name>/):"
	@echo "  codegen-all        Generate code for all examples as submodules"
	@echo "  build-examples     Generate and compile unified binary"
	@echo "  regenerate-imports Update generated/imports.go"
	@for name in $(EXAMPLE_NAMES); do \
		echo "  codegen-$$name"; \
	done
	@echo ""
	@echo "Run targets (uses unified CLI):"
	@for name in $(EXAMPLE_NAMES); do \
		echo "  run-$$name"; \
	done
	@echo ""
	@echo "Serve command:"
	@echo "  ./petri-pilot serve              List available services"
	@echo "  ./petri-pilot serve <name>       Run a specific service"
	@echo ""
	@echo "E2E test targets:"
	@echo "  e2e            Run E2E tests (headless)"
	@echo "  e2e-headed     Run E2E tests with browser visible"
	@echo "  e2e-debug      Run E2E tests with debug output"
	@echo ""
	@echo "Other targets:"
	@echo "  mcp            Run the MCP server"
	@echo "  generate       Generate model from requirements (REQ='...')"
	@echo ""
	@echo "Examples:"
	@echo "  make validate-order-processing"
	@echo "  make codegen-erc20-token"
	@echo "  make run-order-processing"
	@echo "  make generate REQ='order processing workflow'"
