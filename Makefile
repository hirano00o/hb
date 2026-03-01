.PHONY: build test lint clean

BINARY := hb
CMD_DIR := ./cmd/hb

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o $(BINARY) $(CMD_DIR)

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -f $(BINARY)
