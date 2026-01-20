.PHONY: lint build clean
.PHONY: test test-verbose test-race test-cover

BINARY_NAME := hnkeep
VERSION     ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT      ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
LDFLAGS     := -ldflags "-X main.version=$(VERSION) -X main.commit=$(COMMIT)"

lint:
	go vet ./...
	go fmt ./...
	gofmt -s -w .

test:
	go test ./...
test-verbose:
	go test -v ./...
test-race:
	go test -race ./...
test-cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

build:
	go build $(LDFLAGS) -o $(BINARY_NAME) ./cmd/hnkeep

clean:
	rm -f $(BINARY_NAME)
