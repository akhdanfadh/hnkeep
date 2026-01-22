# Learning by Coding (journal)

Periodically, I document what I learn in this journal.
My learning entries are first written in the code itself with `NOTE:` comment tag.
When it is not relevant anymore, I move it here for future reference.
Most likely when the project is ready for release, I will move them all here.

The following notes are sorted by the most recent at the top.

## time.After memory leak before Go 1.23

```go
// internal/hackernews/client.go (e3f3ffd, 2026-01-22)

// waitWithContext waits for the specified duration or until context is cancelled.
//
// NOTE: In previous commit, we use time.After that, until Go 1.23, allocates a timer
// that won't be GC'd until it fires. If the context is cancelled early, the timer
// lives until expiry, creating memory pressure for long durations (e.g., 30s backoff).
// The solution is to use time.NewTimer with explicit Stop(), and we do that here for clarity.
// - https://pkg.go.dev/time#After (see "underlying Timer would not be recovered")
func waitWithContext(ctx context.Context, d time.Duration) error {
    timer := time.NewTimer(d)
    select {
    case <-ctx.Done():
        timer.Stop()
        return ctx.Err()
    case <-timer.C:
        return nil
    }
}
```

## Go select random case selection edge case

```go
// internal/converter/converter.go (b97ac4a, 2026-01-22)

// NOTE: This check handles an edge case in Go's select behavior.
// When multiple cases are ready simultaneously, select picks one via
// "uniform pseufo-random selection". So if the context is cancelled
// at the exact moment a semaphore slot opens, we might acquire the
// semaphore instead of exiting via ctx.Done(). This is defensive.
// - https://golang.org/ref/spec#Select_statements.
if ctx.Err() != nil {
    return
}
```

## Clarification: ctx check before channel send

```go
// internal/converter/converter.go (b97ac4a, 2026-01-22)

item, err := c.fetcher.GetItem(ctx, bookmark.ID)
// NOTE: My comment was wrong on this. I put "to avoid blocking on full channel"
// because checking ctx before channel sends is a common pattern to prevent
// goroutine leaks. But actually that's wrong here. The result channel is sized
// to len(bookmarks), and each goroutine sends at most one result, so it can never
// be full. The actual purpose is to skip unnecessary work after cancellation,
// like sending or logging.
if ctx.Err() != nil {
    return
}
```

## Error Unwrap convention for error chains

```go
// internal/syncer/syncer.go (2960fb1, 2026-01-22)

// Unwrap returns the underlying error for use with errors.Is and errors.As.
//
// NOTE: Unwrap is part of Go's error wrapping convention. By implementing this,
// we allow callers to inspect the underlying error using errors.Is(err, target)
// and errors.As(err, &target) to extract typed errors from the chain.
// Without Unwrap, a SyncError would be opaque and callers couldn't check what
// caused the sync failure. This is important for graceful shutdown detection.
// - https://go.dev/blog/go1.13-errors
// - https://pkg.go.dev/errors#Unwrap
func (e SyncError) Unwrap() error {
    return e.Err
}
```

## Short-circuit evaluation with || and nil checks

```go
// internal/syncer/syncer.go (71055b8, 2026-01-22)

// NOTE: Short-circuit evaluation with || ensures we only dereference
// after the nil check fails, avoiding a nil pointer dereference panic.
// Silly beginner mistake by me, mine was reversed before this.
if incoming == nil || *incoming == "" {
    return existing, false
}
```

## Go iota for enums

```go
// internal/syncer/syncer.go (71055b8, 2026-01-22)

// NOTE: Finally, yes, we are using enums! The terms "iota" itself is a letter
// in the Greek alphabet meaning "smallest" or "least" and typical for math notations.
// - https://go.dev/wiki/Iota
// - https://stackoverflow.com/questions/14426366/what-is-an-idiomatic-way-of-representing-enums-in-go
// - https://stackoverflow.com/questions/31650192/whats-the-full-name-for-iota-in-golang

// SyncStatus represents the result of a sync operation.
type SyncStatus int

const (
    SyncFailed SyncStatus = iota
    SyncCreated
    SyncUpdated
    SyncSkipped
)
```

## Karakeep API always expects JSON

```go
// internal/karakeep/client.go (71055b8, 2026-01-22)

// NOTE: Karakeep API (built with Hono) always expects JSON request bodies
// (validated via zValidator("json", ...)) and returns JSON responses via c.json().
// Errors are returned as JSON via HTTPException with { message: string } format.
req.Header.Set("Authorization", "Bearer "+c.apiKey)
if body != nil {
    req.Header.Set("Content-Type", "application/json")
}
req.Header.Set("Accept", "application/json")
```

## Sentinel errors naming origin

```go
// internal/karakeep/types.go (71055b8, 2026-01-22)

// NOTE: The term sentinel for "sentinel errors" comes from the concept of a
// sentinel value in CS, which is a special value that marks a boundary or
// signals a particular condition. Like a guard (sentinel) standing watch,
// these errors "stand out" as recognizable markers for specific situations,
// rather than being generic error messages that require string parsing to
// undertand. In Go, they are typically defined as package-level variables
// of type error and compared using errors.Is().

// Sentinel errors for specific API conditions.
var (
    ErrUnauthorized     = errors.New("unauthorized: invalid or missing API key")
    ErrBookmarkNotFound = errors.New("bookmark not found")
    ErrRateLimited      = errors.New("rate limited: too many requests")
)
```

## HTTPError stores raw body for varied error formats

```go
// internal/karakeep/types.go (71055b8, 2026-01-22)

// HTTPError represents an HTTP error from the API with status code and response body.
//
// NOTE: We store the raw Body string rather than parsing a specific JSON structure because
// Karakeep's error responses vary. Storing raw body ensures we capture all error details for debugging.
// - Format 1: manual JSON response (search `c.json` in packages/api/routes/*.ts)
// - Format 2: Hono's HTTP exception (search `throw new HTTPException` in packages/api/middlewares/*.ts)
// - Format 3: Hono's Zod validation errors (search `zValidator" in packages/api/routes/*.ts)
// For reference, Hono is the web framework used by Karakeep: https://hono.dev/.
type HTTPError struct {
    StatusCode int
    Body       string
}
```

## Go versioning via ldflags

```go
// cmd/hnkeep/main.go (8b0b94f4, 2026-01-20)

// NOTE: Go versioning via ldflags is the standard pattern for CLI tools.
// The linker's -X flag sets string variables at build time without modifying source code.
// This enables:
// - `make build`:
//   Makefile injects git tag/commit via ldflags (clean, controlled format)
// - `go build` / `go install ...@version`:
//   Falls back to runtime/debug.ReadBuildInfo() which Go populates automatically
//   with module version info (pseudo-version for untagged: v0.0.0-YYYYMMDD-commit)
// For releases, tag with semver (git tag v1.0.0) so both ldflags and go install work correctly.
// - https://blog.cloudflare.com/setting-go-variables-at-compile-time/
// - https://pkg.go.dev/runtime/debug#ReadBuildInfo
```

## Context propagation for graceful shutdown

```go
// cmd/hnkeep/main.go (dea66c6f, 2026-01-20)

func main() {
    // NOTE: Root context that cancels on SIGINT (Ctrl+C) or SIGTERM.
    // This enables graceful shutdown: all in-flight operations will be cancelled
    // and goroutines can exit cleanly instead of being forcefully terminated.
    // Context itself is basically a way in Go to propagate signals across concurrent operations.
    // - https://medium.com/@jamal.kaksouri/the-complete-guide-to-context-in-golang-efficient-concurrency-management-43d722f6eaea
    ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
    defer cancel()
    ...
}
```

## Type assertion in Go

```go
// internal/cli/cli.go (f8e5c6cf, 2026-01-20)

// NOTE: This is a type assertion in Go. It checks if the interface
// value `fetcher` holds the concrete type `*hackernews.CachedClient`.
// - https://go.dev/doc/effective_go#interface_conversions
if cc, ok := fetcher.(*hackernews.CachedClient); ok {
    stats.cacheHits = cc.CacheHits()
}
```

## Singleflight concurrency control

```go
// internal/hackernews/cache.go (0d332bf6, 2026-01-19)

// NOTE: This is a simplified "singleflight" concurrency control implementation.
// It deduplicates concurrent requests for the same key (item ID in our case)
// so only one fetch happens while others wait for the result.
// If not configured, multiple goroutines requesting the same item ID could all
// miss cache, all fetch from the API, and all write to the same file concurrently.
// - https://pkg.go.dev/golang.org/x/sync/singleflight
```

## Null object pattern

```go
// internal/converter/converter.go (c9904edf, 2026-01-19)

// noopLogger is a Logger implementation that does nothing.
// It silently discards all messages without writing them anywhere.
type noopLogger struct{}

// NOTE: This is a common pattern in Go called the "null object pattern",
// i.e., providing a valid, do-nothing implementation instead of using nil.

func (noopLogger) Info(string, ...any)  {}
func (noopLogger) Warn(string, ...any)  {}
func (noopLogger) Error(string, ...any) {}
```

## HN API quirk: always returns 200 OK

```go
// internal/hackernews/client.go (8b91ce77, 2026-01-19)

// NOTE: Turns out HN API always returns 200 OK (probably Firebase quirk, idk).
// For non-existent items, it returns "null" in the body.
if item.ID == 0 {
    return nil, ErrItemNotFound
}
```

## Closure capture issue in goroutines

```go
// internal/converter/converter.go (24b6e5a1, 2026-01-19)

for _, bm := range bookmarks {
    wg.Add(1)
    // NOTE: We need to pass bm as parameter to avoid closure capture issue.
    // Otherwise, all goroutines would capture the same loop variable reference (last value in loop).
    // - https://go.dev/wiki/CommonMistakes
    // - https://go.dev/doc/faq#closures_and_goroutines
    go func(bookmark harmonic.Bookmark) {
        ...
    }(bm)
}
```

## Concurrency pattern with semaphores

```go
// internal/converter/converter.go (24b6e5a1, 2026-01-19)

func (c *Converter) FetchItems(...) ... {
    ...
    // NOTE: Having read "Grokking Concurrency" really helped me understand this concurrency pattern.
    var wg sync.WaitGroup
    for _, bm := range bookmarks {
        wg.Add(1)
        go func(bookmark harmonic.Bookmark) {
            ...
        }(bm)
    }
    ...
}
```

## Go does not support constant arrays/maps/slices

```go
// internal/converter/converter.go (24b6e5a1, 2026-01-19)

// NOTE: Go does not support constant arrays, maps, or slices.
// - https://blog.boot.dev/golang/golang-constant-maps-slices
// - https://stackoverflow.com/questions/13137463/declare-a-constant-array
```

## JSON struct tags with "-"

```go
// internal/karakeep/types.go (24b6e5a1, 2026-01-19)

// NOTE: JSON struct tags with "-" tells the encoder/decoder to ignore these fields. This ensures that:
// - Marshal: only use our custom logic runs (yes, our custom marshaler implement the Marshaler interface)
// - Unmarshal: Go doesn't try to unmarshal into all fields, only the relevant one based on "type".
```

## JSON omitempty and pointers (nullable)

```go
// internal/karakeep/types.go (24b6e5a1, 2026-01-19)

// NOTE: On when to use omitempty and pointers (nullable).
// Use pointers for fields that are explicitly nullable in the schema.
// Pointers let you distinguish between null (nil) vs zero value vs missing.
// Use omitempty for fields that should be omitted from JSON when they have zero/nil value.
// Remember: JSON null, "", and missing field are different concepts.
// - null: pointer is nil
// - "": pointer to empty string
// - missing: depends on omitempty (omitted if present, or nil/zero if absent)
```

## HTTP response body close errors

```go
// internal/hackernews/client.go (7cc4f161, 2026-01-19)

func (c *Client) fetchItem(ctx context.Context, url string) (*Item, error) {
    ...
    // NOTE: Close errors are not actionable here. The response body has already been
    // read and the actual HTTP operation succeeded or failed. Network errors during
    // close are transient and don't indicate application logic issues.
    defer func() { _ = resp.Body.Close() }()
    ...
}
```

## Functional options pattern

```go
// internal/hackernews/client.go (7cc4f161, 2026-01-19)

func NewClient(opts ...ClientOption) *Client {
    c := &Client{...}
    // NOTE: Functional options pattern: allows callers to customize behavior
    // (e.g., in tests) while keeping NewClient() clean and simple for common case.
    for _, opt := range opts {
        opt(c)
    }
    return c
}
```

## Clever separator design

```go
// internal/harmonic/parser.go (8eeb1d54, 2026-01-18)

func Parse(input string) ([]Bookmark, error) {
    ...
    // NOTE: Surely using '-' as separator is clever here, no? As we can
    // eliminate the case for negative numbers right away. Using any
    // non-numeric character would work equally well, but this is good design.
    parts := strings.Split(input, "-")
    ...
}
```

## Table-driven tests

```go
// internal/harmonic/parser_test.go (8eeb1d54, 2026-01-18)

// NOTE: We are implementing table-driven tests here following
// https://dave.cheney.net/2019/05/07/prefer-table-driven-tests
```

## Variable shadowing bug with named returns

```go
// internal/cli/cli.go (0c5e478f, 2026-01-18)

func writeOutput(path string, export karakeep.Export) (err error) {
    ...
    if path != "" {
        // NOTE: I wrote a bug here by using `err :=` which shadowed the named return
        // `err` the defer needs to report Close() errors. So be careful with that.
        f, createErr := os.Create(path)
        ...
    }
    ...
}
```

## Go string immutability and efficient concatenation

```go
// internal/cli/cli.go (8eeb1d5, 2026-01-18)

func Run() error {
    ...
    // NOTE: Go strings are immutable, so using string concatenation in a loop
    // can lead to excessive memory allocations (a hint from `go vet`).
    // - https://stackoverflow.com/questions/1760757/how-to-efficiently-concatenate-strings-in-go/47798475#47798475.
    var output strings.Builder
    for _, bm := range bookmarks {
        fmt.Fprintf(&output, "%d %d\n", bm.ID, bm.Timestamp)
    }
    ...
}
```

## POSIX standard for text files (trailing newline)

````go
// internal/cli/cli.go (80a9cb5, 2026-01-18)

func Run() error {
    ...

    // NOTE: You may see a new blank line appended before the closing backtick
    // eventhough you may not see any newline char in the input files in your IDE.
    // This is how the POSIX standard for text files.
    // You need to use a hex editor to see it (`cat -A <file>` or `od -c <file>`).
    // - https://stackoverflow.com/questions/729692/why-should-text-files-end-with-a-newline
    output := fmt.Sprintf("You are reading the output file. Here is a copy of your input:\n\n```\n%s\n```", input)
    if err := writeOutput(cfg.OutputPath, output); err != nil {
        return fmt.Errorf("writing output: %w", err)
    }


    ...
}
````

## UNIX filter behavior

````go
// internal/cli/cli.go (80a9cb51, 2026-01-18)

func Run(ctx context.Context) error {
    ...
    // if no input data is given and stdin is a terminal, show usage and exit
    // NOTE: Without this check, it "feels" like the program is hanging. That is actually a
    // standard UNIX filter behavior (see example below). But for better UX we show usage instead.
    // Example (like cat, grep, sed, awk):
    // ```
    // $ ./hnkeep
    // hello world      <-- you type this
    // ^D               <-- Ctrl+D to send EOF
    // <output appears here>
    // ```
    if cfg.InputPath == "" {
        if stat, _ := os.Stdin.Stat(); (stat.Mode() & os.ModeCharDevice) != 0 {
            flag.Usage()
            return nil
        }
    }
    ...
}
````

## bufio.Writer usage

```go
// internal/cli/cli.go (80a9cb51, 2026-01-18)

func writeOutput(path string, export karakeep.Export) (err error) {
    // NOTE: Use bufio.Writer here if you are making many small writes and want to avoid
    // overhead of frequent syscalls. However, we are writing only once in this code.
    // - https://pkg.go.dev/bufio#Writer
    ...
}
```

## Go flag package doesn't support aliases natively

```go
// internal/cli/cli.go (80a9cb5, 2026-01-18)

// NOTE: go flag package does not support alias natively.
// - https://github.com/golang/go/issues/35761
```
