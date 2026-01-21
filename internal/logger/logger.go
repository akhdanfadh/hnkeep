package logger

import (
	"fmt"
	"io"
	"sync"
)

// Logger defines the interface for logging messages.
type Logger interface {
	Info(format string, args ...any)
	Warn(format string, args ...any)
	Error(format string, args ...any)
}

// Noop returns a do-nothing Logger (null object pattern).
func Noop() Logger { return &noopLogger{} }

type noopLogger struct{}

func (noopLogger) Info(string, ...any)  {}
func (noopLogger) Warn(string, ...any)  {}
func (noopLogger) Error(string, ...any) {}

// StdLogger provides thread-safe structured logging to an output writer.
type StdLogger struct {
	mu    sync.Mutex
	out   io.Writer
	quiet bool
}

// NewStdLogger creates a new Logger that writes to the given writer.
// If quiet is true, Info messages are suppressed.
func NewStdLogger(out io.Writer, quiet bool) *StdLogger {
	return &StdLogger{
		out:   out,
		quiet: quiet,
	}
}

// Info logs an informational message with [INFO] prefix.
func (l *StdLogger) Info(format string, args ...any) {
	if l.quiet {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = fmt.Fprintf(l.out, "[INFO] "+format+"\n", args...)
}

// Warn logs an informational message with [WARN] prefix.
func (l *StdLogger) Warn(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = fmt.Fprintf(l.out, "[WARN] "+format+"\n", args...)
}

// Error logs an informational message with [ERROR] prefix.
func (l *StdLogger) Error(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, _ = fmt.Fprintf(l.out, "[ERROR] "+format+"\n", args...)
}
