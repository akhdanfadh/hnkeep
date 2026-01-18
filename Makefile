.PHONY: lint build clean
.PHONY: test test-verbose test-race test-cover

BINARY_NAME := hnkeep

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
	go build -o $(BINARY_NAME) ./cmd/hnkeep

clean:
	rm -f $(BINARY_NAME)
