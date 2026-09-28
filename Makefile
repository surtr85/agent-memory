.PHONY: all build test clean

BINARY_NAME=agent-memory
BIN_DIR=bin

all: build

build:
	mkdir -p $(BIN_DIR)
	CGO_ENABLED=0 go build -ldflags="-s -w" -o $(BIN_DIR)/$(BINARY_NAME) ./cmd/agent-memory

test:
	go test -count=1 -v ./...

clean:
	rm -rf $(BIN_DIR)
