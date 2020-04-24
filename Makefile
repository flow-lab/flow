SHELL := /bin/bash -e

.DEFAULT_GOAL := help

.PHONY: deps d
deps d: ## Download dependencies
	@go mod download

.PHONY: deps-upgrade du
deps-upgrade du: ## Upgrade all dependencies
	go get -u -t ./...

.PHONY: tidy ti
tidy ti: ## Tidy
	@go mod tidy

.PHONY: test t
test t: ## Run tests
	@go test -covermode=atomic -installsuffix "static" ./...

.PHONY: build b
build b: ## Build
	@go build ./cmd/flow

.PHONY: install i
install i: ## Install
	@go install ./...

.PHONY: generate
generate g: ## Run mockgen
	@go generate -run="mockgen" ./...

.PHONY: help
help: ## Show this
	@grep -E '^[a-zA-Z_ -]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-16s\033[0m %s\n", $$1, $$2}'