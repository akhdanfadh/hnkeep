package cli

import (
	"fmt"
	"io"
	"sync"
)

// Logger provides thread-safe structured logging to an output writer.
type Logger struct {
	mu    sync.Mutex
	out   io.Writer
	quiet bool
}

// NewLogger creates a new Logger that writes to the given writer.
// If quiet is true, Info messages are suppressed.
func NewLogger(out io.Writer, quiet bool) *Logger {
	return &Logger{
		out:   out,
		quiet: quiet,
	}
}

// Info logs an informational message with [INFO] prefix.
func (l *Logger) Info(format string, args ...any) {
	if l.quiet {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = fmt.Fprintf(l.out, "[INFO] "+format+"\n", args...)
}

// Warn logs an informational message with [WARN] prefix.
func (l *Logger) Warn(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = fmt.Fprintf(l.out, "[WARN] "+format+"\n", args...)
}

// Error logs an informational message with [ERROR] prefix.
func (l *Logger) Error(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = fmt.Fprintf(l.out, "[ERROR] "+format+"\n", args...)
}
