.DEFAULT_GOAL := help
.PHONY: help fmt build test structure-test check-renovate iso all

IMAGE ?= proves-ci-os:dev

help: ## Show this help
	@awk 'BEGIN {FS = ":.*## "} /^[a-z-]+:.*## / {printf "%-16s %s\n", $$1, $$2}' $(MAKEFILE_LIST)

fmt: ## Run every pre-commit hook on all files
	pre-commit run --all-files

build: ## Build the Image with podman (CI is the authoritative build)
	podman build -t $(IMAGE) .

test: ## Run Go tests
	go test ./...

structure-test: ## Run container-structure-test against $(IMAGE) (loaded into docker)
	podman save $(IMAGE) | docker load
	container-structure-test test --image localhost/$(IMAGE) --platform linux/amd64 \
		$(foreach f,$(wildcard tests/structure/*.yaml),--config $(f))

check-renovate: ## Assert every pinned ARG has one manager and validate renovate.json
	go run ./cmd/check-renovate-pins
	renovate-config-validator --strict

ISO_PAYLOAD ?= ghcr.io/open-source-space-foundation/proves-ci-os:latest

iso: ## Build the installer ISO into ./out (needs root, podman and image-builder; see ADR 0008)
	podman build -t localhost/proves-ci-os-installer -f iso/Containerfile iso
	podman pull $(ISO_PAYLOAD)
	image-builder build --bootc-ref localhost/proves-ci-os-installer \
		--bootc-installer-payload-ref $(ISO_PAYLOAD) --bootc-default-fs ext4 \
		--output-dir out --output-name proves-ci-os-installer bootc-generic-iso

all: fmt build test structure-test check-renovate ## Everything except iso
