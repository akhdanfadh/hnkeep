package logger

import (
	"bytes"
	"strings"
	"sync"
	"testing"
)

func TestLoggerInfo(t *testing.T) {
	var buf bytes.Buffer
	logger := NewStdLogger(&buf, false)
	logger.Info("test message: %s", "hello")

	got := buf.String()
	want := "[INFO] test message: hello\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLoggerWarn(t *testing.T) {
	var buf bytes.Buffer
	logger := NewStdLogger(&buf, false)
	logger.Warn("test message: %s", "hello")

	got := buf.String()
	want := "[WARN] test message: hello\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLoggerError(t *testing.T) {
	var buf bytes.Buffer
	logger := NewStdLogger(&buf, false)
	logger.Error("test message: %s", "hello")

	got := buf.String()
	want := "[ERROR] test message: hello\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestLoggerQuietMode(t *testing.T) {
	var buf bytes.Buffer
	logger := NewStdLogger(&buf, true)
	logger.Info("this should be suppressed")
	logger.Warn("this should appear")
	logger.Error("this should also appear")

	got := buf.String()
	if strings.Contains(got, "this should be suppressed") {
		t.Errorf("Info message was not suppressed in quiet mode")
	}
	if !strings.Contains(got, "this should appear") {
		t.Errorf("Warn message was not logged")
	}
	if !strings.Contains(got, "this should also appear") {
		t.Errorf("Error message was not logged")
	}
}

func TestLoggerConcurrentWrites(t *testing.T) {
	var buf bytes.Buffer
	logger := NewStdLogger(&buf, false)

	var wg sync.WaitGroup
	iterations := 100

	// launch concurrent logging
	for i := range iterations {
		wg.Add(3)
		go func(n int) {
			defer wg.Done()
			logger.Info("info %d", n)
		}(i)
		go func(n int) {
			defer wg.Done()
			logger.Warn("warn %d", n)
		}(i)
		go func(n int) {
			defer wg.Done()
			logger.Error("error %d", n)
		}(i)
	}
	wg.Wait()

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	// verify all messages were written
	expectedLines := iterations * 3
	if len(lines) != expectedLines {
		t.Errorf("got %d log lines, want %d", len(lines), expectedLines)
	}

	// verify no corrupted lines
	for i, line := range lines {
		if !strings.HasPrefix(line, "[INFO]") &&
			!strings.HasPrefix(line, "[WARN]") &&
			!strings.HasPrefix(line, "[ERROR]") {
			t.Errorf("line %d appears corrupted: %q", i, line)
		}
	}
}
