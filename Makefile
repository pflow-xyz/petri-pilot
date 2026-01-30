.PHONY: test dev-run help

# Default target
.DEFAULT_GOAL := help

# Binary name
BINARY := petri-pilot

# Run all tests
test:
	go test ./...

# Run dev server (used on pilot.pflow.xyz)
dev-run: build
	./$(BINARY) serve -port 8083 tic-tac-toe zk-tic-tac-toe coffeeshop

# Build the binary
build:
	go build -o $(BINARY) ./cmd/petri-pilot

# Help target
help:
	@echo "Petri Pilot Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  test      Run all tests"
	@echo "  dev-run   Build and run the dev server"
	@echo "  build     Build the petri-pilot binary"
