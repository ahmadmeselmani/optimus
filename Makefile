SHELL := /bin/bash

BINARY    := optimus
BIN_DIR   := bin
CMD       := ./cmd/optimus
HANDBOOK  := handbook
NVM_SH    := $(HOME)/.nvm/nvm.sh

# Make Node/npm available for handbook targets even when they're only
# installed via nvm and not on PATH by default.
NODE_ENV := if ! command -v npm >/dev/null 2>&1 && [ -f $(NVM_SH) ]; then source $(NVM_SH) && nvm use >/dev/null; fi

.DEFAULT_GOAL := help

.PHONY: help build run test vet fmt fmt-check clean check \
        handbook-install handbook-dev handbook-build handbook-serve handbook-clean

# Free-text argument passthrough for `run`: words typed after "run"
# (e.g. `make run ask hello world`) become $(RUN_ARGS) instead of Make
# treating them as goals of their own. This relies on the catch-all rule
# at the bottom of this file to silently no-op those words as goals; that
# also means a real target name typed as an "argument" runs redundantly
# (harmless), a typo'd target silently no-ops, and a word containing "="
# is parsed by Make as a variable assignment and dropped.
RUN_ARGS := $(wordlist 2,$(words $(MAKECMDGOALS)),$(MAKECMDGOALS))

help: ## Show this help
	@echo "Optimus — available targets:"
	@grep -hE '^[a-zA-Z0-9_-]+:.*##' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*## "}; {printf "  %-18s %s\n", $$1, $$2}'

build: ## Build the optimus binary into bin/optimus
	go build -o $(BIN_DIR)/$(BINARY) $(CMD)

run: ## go run the CLI directly, e.g. make run ask hello world (no quotes needed)
	go run $(CMD) $(RUN_ARGS)

test: ## Run go test ./...
	go test ./...

vet: ## Run go vet ./...
	go vet ./...

fmt: ## Reformat Go source with gofmt
	gofmt -w .

fmt-check: ## Fail if any Go source is not gofmt'd
	@unformatted=$$(gofmt -l .); \
	if [ -n "$$unformatted" ]; then \
		echo "not gofmt'd:"; echo "$$unformatted"; exit 1; \
	fi

clean: ## Remove build artifacts
	rm -rf $(BIN_DIR)

handbook-install: ## Install handbook (Docusaurus) dependencies
	@cd $(HANDBOOK) && $(NODE_ENV) && npm install

handbook-dev: ## Run the handbook dev server
	@cd $(HANDBOOK) && $(NODE_ENV) && npm run start

handbook-build: ## Build the static handbook site (must pass before a milestone is "done")
	@cd $(HANDBOOK) && $(NODE_ENV) && npm run build

handbook-serve: handbook-build ## Build and serve the handbook locally
	@cd $(HANDBOOK) && $(NODE_ENV) && npm run serve

handbook-clean: ## Remove handbook build artifacts
	rm -rf $(HANDBOOK)/build $(HANDBOOK)/.docusaurus

check: fmt-check vet test build handbook-build ## Run everything required before committing a milestone
	@echo "All checks passed."

# Catch-all: silently no-op any goal without its own rule above. This is
# what lets `run` take free-text as if it were arguments instead of Make
# erroring with "No rule to make target ...". See RUN_ARGS above.
%:
	@:
