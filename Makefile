GO ?= go
PNPM ?= pnpm
VERSION ?= dev
BIN_DIR ?= bin

.PHONY: build go-build web-build test lint fmt dev clean

build: web-build go-build

go-build:
	mkdir -p $(BIN_DIR)
	$(GO) build -ldflags "-X main.version=$(VERSION)" -o $(BIN_DIR)/nomici ./cmd/nomici

web-build:
	$(PNPM) install --frozen-lockfile
	$(PNPM) --filter @nomici/web build

test: web-build
	$(GO) test ./...
	$(PNPM) --filter @nomici/web test

lint: web-build
	$(GO) vet ./...
	$(PNPM) --filter @nomici/web lint

fmt:
	gofmt -w $$(find cmd internal -name '*.go')
	$(PNPM) install --frozen-lockfile
	$(PNPM) --filter @nomici/web fmt

dev:
	$(GO) run ./cmd/nomici gateway run

clean:
	rm -rf $(BIN_DIR)
