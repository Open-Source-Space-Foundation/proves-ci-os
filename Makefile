.DEFAULT_GOAL := help
.PHONY: help fmt build test check-renovate iso all

IMAGE ?= proves-ci-os:dev

help: ## Show this help
	@awk 'BEGIN {FS = ":.*## "} /^[a-z-]+:.*## / {printf "%-16s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

fmt: ## Run every pre-commit hook on all files
	pre-commit run --all-files

build: ## Build the Image with podman (CI is the authoritative build)
	podman build -t $(IMAGE) .

test: ## Run Go tests
	go test ./...

check-renovate: ## Assert every pinned ARG has one manager and validate renovate.json
	go run ./cmd/check-renovate-pins
	renovate-config-validator --strict

iso: ## Build the installer ISO (see ADR 0008)
	@echo "iso target not implemented yet" >&2; exit 1

all: fmt build test check-renovate ## Everything except iso
