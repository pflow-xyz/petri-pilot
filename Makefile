.PHONY: test test-browser e2e-install dev-run help build

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

# Drive the café console in a real browser: builds, serves on a free port,
# checks, tears down. One command, no second shell.
#
# Needs the browser tooling in e2e/ (`make e2e-install`, once), so it is not part
# of `make test` — a fresh clone should not fail on a missing browser. CI runs it
# as its own job, so the gap is covered where it can be.
#
# Pass BASE=<url> to check a server that is already running instead.
test-browser: build
	@node frontends/cafe/src/console.browser.mjs $(BASE)
	@node frontends/vet-clinic/whatif.browser.mjs $(BASE_VET)

# Just the vet-clinic what-if console check.
test-browser-vet: build
	@node frontends/vet-clinic/whatif.browser.mjs $(BASE_VET)

# Install the browser tooling (Playwright + its Chromium) into e2e/.
e2e-install:
	cd e2e && npm install && npx playwright install --with-deps chromium

# Run dev server (used on pilot.pflow.xyz)
dev-run: build
	./$(BINARY) serve -port 8083 tic-tac-toe zk-tic-tac-toe coffeeshop texas-holdem knapsack predator-prey dining-philosophers loan-approval tcp-handshake thermostat producer-consumer hiring-pipeline enzyme-kinetics stoplight galton-board cafe vet-clinic

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
