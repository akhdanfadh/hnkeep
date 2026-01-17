.PHONY: lint build clean

BINARY_NAME := hnkeep

lint:
	go vet ./...
	go fmt ./...
	gofmt -s -w .

build:
	go build -o $(BINARY_NAME) ./cmd/hnkeep

clean:
	rm -f $(BINARY_NAME)
