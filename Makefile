.PHONY: build run dev lint vet test test-race cover check tidy clean help

## build: compile the server binary to bin/llm-council
build:
	go build -o bin/llm-council ./cmd/server

## run: build and run the compiled binary (reads .env if present)
run: build
	@[ -f .env ] && export $$(grep -v '^#' .env | xargs) || true; ./bin/llm-council

## dev: run the server directly with go run (faster iteration, reads .env if present)
dev:
	@[ -f .env ] && export $$(grep -v '^#' .env | xargs) || true; go run ./cmd/server

## lint: run go vet and staticcheck
lint:
	go vet ./...
	go run honnef.co/go/tools/cmd/staticcheck ./...

## vet: alias for lint
vet: lint

## test: run all tests
test:
	go test ./...

## test-race: run all tests with the race detector
test-race:
	go test -race ./...

## cover: run tests and open an HTML coverage report
cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out
	@rm -f coverage.out

## check: full pre-flight (lint + test-race) — run before opening a PR
check: lint test-race

## tidy: tidy and verify go module dependencies
tidy:
	go mod tidy
	go mod verify

## clean: remove build artifacts and coverage output
clean:
	rm -rf bin/ coverage.out

## help: list available targets
help:
	@grep -E '^## ' Makefile | sed 's/^## //'
