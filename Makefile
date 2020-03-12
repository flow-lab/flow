SHELL := /bin/bash

.DEFAULT_GOAL := help

.PHONY: deps
deps: ## Download dependencies
	@go mod download

.PHONY: deps-upgrade
deps-upgrade: ## Upgrade all dependencies
	go get -u -t ./...

.PHONY: tidy
tidy: ## Tidy
	@go mod tidy

.PHONY: test
test: ## Run tests
	@go test -covermode=atomic -installsuffix "static" ./...

.PHONY: build
build: ## Build
	@go build ./...

.PHONY: install
install: ## Install
	@go install ./...

.PHONY: generate
generate: ## Run mockgen
	@go generate -run="mockgen" ./...

.PHONY: help
help: ## Show this
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-16s\033[0m %s\n", $$1, $$2}'