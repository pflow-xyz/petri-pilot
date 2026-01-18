.PHONY: build test clean validate-all codegen-all build-examples help
.PHONY: $(EXAMPLE_TARGETS) $(CODEGEN_TARGETS)

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
	rm -rf $(OUTPUT_DIR)

# Validate all examples
validate-all: $(EXAMPLE_TARGETS)

# Generate code for all examples
codegen-all: $(CODEGEN_TARGETS)

# Generate and build all examples (verifies generated code compiles)
build-examples: codegen-all
	@echo "=== Building all generated examples ==="
	@for name in $(EXAMPLE_NAMES); do \
		echo "Building $$name..."; \
		cd $(OUTPUT_DIR)/$$name && \
		echo "replace github.com/pflow-xyz/petri-pilot => $$(cd ../.. && pwd)" >> go.mod && \
		GOWORK=off go mod tidy && \
		GOWORK=off go build ./... || exit 1; \
		cd - > /dev/null; \
	done
	@echo "All examples built successfully!"

# Individual validate targets
validate-%: examples/%.json
	@echo "=== Validating $< ==="
	go run ./cmd/petri-pilot/... validate $<

# Individual codegen targets
codegen-%: examples/%.json
	@echo "=== Generating code from $< ==="
	@mkdir -p $(OUTPUT_DIR)/$*
	go run ./cmd/petri-pilot/... codegen -o $(OUTPUT_DIR)/$* $<

# Run MCP server
mcp:
	go run ./cmd/petri-pilot/... mcp

# Generate and auto-validate a model from requirements
generate:
	@if [ -z "$(REQ)" ]; then echo "Usage: make generate REQ='your requirements'"; exit 1; fi
	go run ./cmd/petri-pilot/... generate -auto "$(REQ)"

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
	@echo "  codegen-all    Generate code for all examples"
	@echo "  build-examples Generate and compile all examples"
	@for name in $(EXAMPLE_NAMES); do \
		echo "  codegen-$$name"; \
	done
	@echo ""
	@echo "Other targets:"
	@echo "  mcp            Run the MCP server"
	@echo "  generate       Generate model from requirements (REQ='...')"
	@echo ""
	@echo "Examples:"
	@echo "  make validate-order-system"
	@echo "  make codegen-token-ledger OUTPUT_DIR=./myout"
	@echo "  make generate REQ='order processing workflow'"
