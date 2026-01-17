.PHONY: build test clean validate-all codegen-all help
.PHONY: $(EXAMPLE_TARGETS) $(CODEGEN_TARGETS)

# Binary name
BINARY := petri-pilot

# Example files
EXAMPLES := $(wildcard examples/*.json)
EXAMPLE_NAMES := $(basename $(notdir $(EXAMPLES)))

# Generate target names
EXAMPLE_TARGETS := $(addprefix validate-,$(EXAMPLE_NAMES))
CODEGEN_TARGETS := $(addprefix codegen-,$(EXAMPLE_NAMES))

# Default output directory for codegen
OUTPUT_DIR ?= ./out

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

# Individual validate targets
validate-%: examples/%.json
	@echo "=== Validating $< ==="
	go run ./cmd/petri-pilot/... validate $<

# Individual codegen targets
codegen-%: examples/%.json
	@echo "=== Generating code from $< ==="
	@mkdir -p $(OUTPUT_DIR)/$*
	go run ./cmd/petri-pilot/... codegen $< -o $(OUTPUT_DIR)/$* -lang go

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
