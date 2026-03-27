.PHONY: generate lint-buf test lint vet run-server run-mcp

generate:
	go run github.com/bufbuild/buf/cmd/buf@latest generate

lint-buf:
	go run github.com/bufbuild/buf/cmd/buf@latest lint

test:
	go test ./...

vet:
	go vet ./...

lint: vet lint-buf

run-server:
	go run ./cmd/server

run-mcp:
	go run ./cmd/mcp
