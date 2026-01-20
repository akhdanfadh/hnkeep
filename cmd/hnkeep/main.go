package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"

	"github.com/akhdanfadh/hnkeep/internal/cli"
)

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

// version and commit are set during build time using -ldflags,
var (
	version = "dev"
	commit  = "none"
)

// getVersion returns the application version.
func getVersion() string {
	// if ldflags set a specific version, use it
	if version != "dev" {
		return version
	}
	// otherwise, try to get version from Go module info (go install ...@v1.0.0)
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return version
}

func main() {
	// NOTE: Root context that cancels on SIGINT (Ctrl+C) or SIGTERM.
	// This enables graceful shutdown: all in-flight operations will be cancelled
	// and goroutines can exit cleanly instead of being forcefully terminated.
	// Context itself is basically a way in Go to propagate signals across concurrent operations.
	// - https://medium.com/@jamal.kaksouri/the-complete-guide-to-context-in-golang-efficient-concurrency-management-43d722f6eaea
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cli.Version, cli.Commit = getVersion(), commit
	if err := cli.Run(ctx); err != nil {
		if ctx.Err() != nil {
			fmt.Fprintln(os.Stderr, "\nInterrupted")
			os.Exit(130) // 128 + SIGINT(2), standard exit code for Ctrl+C
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
