.PHONY: fmt build test

fmt:
	go fmt ./...

build:
	go build ./cmd/pg-investigate

test:
	go test ./...
