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

// getCommit returns the commit hash from build info if available.
func getCommit() string {
	// if ldflags set a specific commit, use it
	if commit != "none" {
		return commit
	}
	// try to get commit from Go module build info
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				return setting.Value
			}
		}
	}
	return "unknown"
}

func main() {
	// graceful shutdown: cancels context on SIGINT/SIGTERM
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	cli.Version, cli.Commit = getVersion(), getCommit()
	if err := cli.Run(ctx); err != nil {
		if ctx.Err() != nil {
			fmt.Fprintln(os.Stderr, "\nInterrupted")
			os.Exit(130) // 128 + SIGINT(2), standard exit code for Ctrl+C
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
