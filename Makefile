GO ?= go
export GOTOOLCHAIN := local

.PHONY: build test check
build:
	$(GO) build -trimpath -o bin/sela ./cmd/sela
test:
	$(GO) test -race ./...
check:
	$(GO) vet ./...
	$(GO) test -race ./...
