.PHONY: build run clean test lint tidy

BIN := do-manager

build:
	go build -o $(BIN) .

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
