BINARY    := opentrace
BUILD_DIR := ./bin
CMD_DIR   := ./cmd/opentrace

.PHONY: all build clean tidy test install

## all: Default — build the binary
all: build

## build: Compile to ./bin/opentrace
build:
	@mkdir -p $(BUILD_DIR)
	go build -trimpath -o $(BUILD_DIR)/$(BINARY) $(CMD_DIR)
	@echo "  built → $(BUILD_DIR)/$(BINARY)"

## install: Install binary to $GOPATH/bin
install:
	go install -trimpath $(CMD_DIR)
	@echo "  installed → $(shell go env GOPATH)/bin/$(BINARY)"
	# check where Go installs binaries
	go env GOPATH

	# add it to your PATH — put this in your ~/.bashrc or ~/.zshrc
	export PATH=$PATH:$(go env GOPATH)/bin

	# reload
	source ~/.bashrc
	source ~/.zshrc

## tidy: Tidy go.mod and go.sum
tidy:
	go mod tidy

## test: Run tests
test:
	go test ./... -race -count=1

## clean: Remove build artifacts
clean:
	rm -rf $(BUILD_DIR)

## help: List available targets
help:
	@grep -E '^## ' Makefile | sed 's/## /  /'