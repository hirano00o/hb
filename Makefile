.PHONY: build test lint clean

BINARY := hb
CMD_DIR := ./cmd/hb

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")

build:
	CGO_ENABLED=0 go build -ldflags="-s -w -X github.com/hirano00o/hb/internal/cli.version=$(VERSION)" -trimpath -o $(BINARY) $(CMD_DIR)

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -f $(BINARY)
