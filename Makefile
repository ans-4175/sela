GO ?= go
PYTHON ?= python3
export GOTOOLCHAIN := local

.PHONY: build test check smoke
build:
	$(GO) build -trimpath -o bin/sela ./cmd/sela
test:
	$(GO) test -race ./...
check:
	$(GO) vet ./...
	$(GO) test -race ./...
	$(PYTHON) -m unittest discover -s scripts -p 'test_*.py'
smoke: build
	$(PYTHON) scripts/smoke.py --binary bin/sela
