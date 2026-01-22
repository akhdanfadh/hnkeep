package logger

import (
	"fmt"
	"io"
	"os"
	"sync"
)

// NOTE: TTY stands for "teletypewriter", electromechanical devices from the 1800s that
// could send typed messages over wires. From the 1950s-1970s, teleprinters were the primary
// way humans interacted with computers. When video terminals (like VT-100) replaced them
// in the late 1970s, Unix kept the "TTY" terminology. Today, terminal emulators (xterm,
// iTerm, etc.) are software simulations called "pseudo-terminals" (PTY), but the kernel
// still treats them as TTY devices. The TTY subsystem handles line editing, control codes,
// and signal generation (e.g., Ctrl+C sending SIGINT).
// - https://www.linusakesson.net/programming/tty/
// - https://www.howtogeek.com/428174/what-is-a-tty-on-linux-and-how-to-use-the-tty-command/

// NOTE: Unix treats everything as files, including I/O streams. When a process starts,
// it gets three standard file descriptors: 0 (stdin), 1 (stdout), 2 (stderr). These are
// integers indexing into the kernel's file table. Each file has mode bits describing its
// type: regular file, directory, symbolic link, FIFO/pipe, socket, block device, or
// character device. Terminals are character devices (sequential byte streams), identified
// by os.ModeCharDevice in Go. When you redirect stderr (e.g., 2>/dev/null), the file
// descriptor now points to a regular file or pipe instead of a character device.
// - https://en.wikipedia.org/wiki/File_descriptor
// - https://en.wikipedia.org/wiki/Unix_file_types
// - https://man7.org/linux/man-pages/man3/stdin.3.html

// IsTTY checks if the given file is connected to a terminal.
//
// NOTE: We check os.ModeCharDevice to detect if the file points to a TTY. When a stream
// is redirected to a file or pipe, it becomes a regular file or FIFO, not a character
// device. This matters because:
// 1. Progress indicators use ANSI escape codes (\r, \033[K) that only work on TTYs
// 2. Writing escape codes to files/pipes corrupts output and confuses downstream tools
// 3. Non-TTY contexts (CI, redirected output) should get clean, parseable output
// - https://rderik.com/blog/identify-if-output-goes-to-the-terminal-or-is-being-redirected-in-golang/
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
