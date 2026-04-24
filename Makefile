GO           ?= go
BINARY_NAME  ?= lx
CMD_DIR      := ./cmd/lx
BIN_DIR      := ./bin

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -s -w -X 'github.com/rasros/lx/internal/cli.Version=$(VERSION)'

.PHONY: all build clean demo

all: build

build:
	@mkdir -p $(BIN_DIR)
	$(GO) build -ldflags="$(LDFLAGS)" -o $(OR_OUT) $(CMD_DIR)

OR_OUT ?= $(BIN_DIR)/$(BINARY_NAME)

install:
	$(GO) install $(CMD_DIR)

fmt:
	$(GO) fmt ./...
	$(GO) mod tidy
	$(GO) vet ./...

test:
	$(GO) test ./...

test-update:
	$(GO) test ./cmd/lx -update

run: build
	$(BIN_DIR)/$(BINARY_NAME)

demo: build
	@command -v vhs >/dev/null || { echo "vhs not found — install from https://github.com/charmbracelet/vhs"; exit 1; }
	PATH="$(CURDIR)/$(BIN_DIR):$$PATH" vhs demo/demo.tape

clean:
	rm -rf $(BIN_DIR)
