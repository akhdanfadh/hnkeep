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

# sample n entries from file randomly, useful for testing
sample-input:
	@if [ -z "$$i" ]; then \
		echo "Usage: make sample-input i=<input-file> [n=<count>]"; \
		exit 1; \
	fi; \
	total=$$(tr '-' '\n' < "$$i" | wc -l | tr -d ' '); \
	n=$${n:-$$total}; \
	tr '-' '\n' < "$$i" | \
	awk -v n="$$n" 'BEGIN{srand()} {lines[NR]=$$0} END{ \
		for(i=1; i<=n && i<=NR; i++) { \
			idx = int(rand()*(NR-i+1))+i; \
			tmp=lines[i]; lines[i]=lines[idx]; lines[idx]=tmp; \
		} \
		for(i=1; i<=n && i<=NR; i++) \
			printf "%s%s", lines[i], (i<n && i<NR?"-":"\n") \
	}'

clean:
	rm -f $(BINARY_NAME)
