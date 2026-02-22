.PHONY: test dev-run help build

# Default target
.DEFAULT_GOAL := help

# Binary name
BINARY := petri-pilot

# Version (overridden by goreleaser or CI)
VERSION ?= dev
LDFLAGS := -s -w -X github.com/pflow-xyz/petri-pilot/internal/version.Version=$(VERSION)

# Run all tests
test:
	go test ./...

# Run dev server (used on pilot.pflow.xyz)
dev-run: build
	./$(BINARY) serve -port 8083 tic-tac-toe zk-tic-tac-toe coffeeshop texas-holdem knapsack predator-prey dining-philosophers loan-approval tcp-handshake thermostat producer-consumer hiring-pipeline enzyme-kinetics stoplight

# Build the binary
build:
	go build -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/petri-pilot

# Help target
help:
	@echo "Petri Pilot Makefile"
	@echo ""
	@echo "Targets:"
	@echo "  test      Run all tests"
	@echo "  dev-run   Build and run the dev server"
	@echo "  build     Build the petri-pilot binary"
	@echo ""
	@echo "Variables:"
	@echo "  VERSION   Build version (default: dev)"
