.PHONY: build run clean test lint tidy

BIN     := do-manager
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT  := $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
DATE    := $(shell date -u +"%Y-%m-%dT%H:%M:%SZ")
LDFLAGS := -X 'github.com/franckferman/do-manager/cmd.Version=$(VERSION)' \
           -X 'github.com/franckferman/do-manager/cmd.Commit=$(COMMIT)' \
           -X 'github.com/franckferman/do-manager/cmd.BuildDate=$(DATE)'

build:
	go build -ldflags="$(LDFLAGS)" -o $(BIN) .

run:
	go run . $(ARGS)

test:
	go test ./... -v

lint:
	go vet ./...

tidy:
	go mod tidy

clean:
	rm -f $(BIN)
