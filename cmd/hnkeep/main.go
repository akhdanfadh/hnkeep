package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/akhdanfadh/hnkeep/internal/cli"
)

func main() {
	// NOTE: Root context that cancels on SIGINT (Ctrl+C) or SIGTERM.
	// This enables graceful shutdown: all in-flight operations will be cancelled
	// and goroutines can exit cleanly instead of being forcefully terminated.
	// Context itself is basically a way in Go to propagate signals across concurrent operations.
	// - https://medium.com/@jamal.kaksouri/the-complete-guide-to-context-in-golang-efficient-concurrency-management-43d722f6eaea
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := cli.Run(ctx); err != nil {
		if ctx.Err() != nil {
			fmt.Fprintln(os.Stderr, "\nInterrupted")
			os.Exit(130) // 128 + SIGINT(2), standard exit code for Ctrl+C
		}
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
