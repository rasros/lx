BINARY_NAME  := lx
CMD_DIR      := ./cmd/lx
BIN_DIR      := ./bin

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X 'github.com/rasros/lx/internal/cli.Version=$(VERSION)'

.PHONY: all build clean

all: build

build:
	@mkdir -p $(BIN_DIR)
	go build -ldflags="$(LDFLAGS)" -o $(OR_OUT) $(CMD_DIR)

OR_OUT ?= $(BIN_DIR)/$(BINARY_NAME)