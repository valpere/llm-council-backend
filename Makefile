.PHONY: build run dev lint vet test clean

build:
	go build -o bin/llm-council ./cmd/server

run: build
	./bin/llm-council

dev:
	go run ./cmd/server

lint:
	go vet ./...
	go run honnef.co/go/tools/cmd/staticcheck ./...

vet: lint

test:
	go test ./...

clean:
	rm -rf bin/
