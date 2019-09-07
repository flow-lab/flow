deps:
	@go mod download

tidy:
	@go mod tidy

test: generate
	@go test -covermode=atomic -installsuffix "static" ./...

build:
	@go build ./...

install:
	@go install ./...

generate:
	@go generate -run="mockgen" ./...
