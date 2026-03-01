.PHONY: build test lint clean

BINARY := hb
CMD_DIR := ./cmd/hb

build:
	go build -o $(BINARY) $(CMD_DIR)

test:
	go test ./...

lint:
	go vet ./...

clean:
	rm -f $(BINARY)
