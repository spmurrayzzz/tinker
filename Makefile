.PHONY: all build test test-all format vet validate deps clean

all: build

build:
	go build -o tinker ./cmd/tinker

test:
	go test ./... -v

format:
	go fmt ./...

vet:
	go vet ./...

validate: format vet test

deps:
	go mod download
	go mod verify

clean:
	rm -f tinker
