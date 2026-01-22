package logger

import (
	"fmt"
	"io"
	"os"
	"sync"
)

// IsTTY checks if the given file is connected to a terminal.
func IsTTY(f *os.File) bool {
	stat, err := f.Stat()
	return err == nil && (stat.Mode()&os.ModeCharDevice) != 0
}

// IsStderrTTY checks if stderr is connected to a terminal.
func IsStderrTTY() bool {
	return IsTTY(os.Stderr)
}

// Progresser defines the interface for reporting progress.
type Progresser interface {
	Update(current, total int)
}

// TTYProgresser provides in-place progress updates to a writer.
type TTYProgresser struct {
	mu     sync.Mutex // protects concurrent writes
	out    io.Writer
	format string
}

// NewProgresser creates a Progresser that writes to the given writer.
// Format should include two %d placeholders for current and total (e.g., "Fetching: %d/%d").
func NewProgresser(out io.Writer, format string) *TTYProgresser {
	return &TTYProgresser{out: out, format: format}
}

// Update updates the progress display in place.
func (p *TTYProgresser) Update(current, total int) {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, _ = fmt.Fprintf(p.out, "\r"+p.format, current, total)
}

// Clear clears the progress line using ANSI escape codes.
func (p *TTYProgresser) Clear() {
	p.mu.Lock()
	defer p.mu.Unlock()
	// \r moves cursor to start of line, \033[K erases from cursor to end of line
	_, _ = fmt.Fprintf(p.out, "\r\033[K")
}
