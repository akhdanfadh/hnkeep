package cli

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/akhdanfadh/hnkeep/internal/hackernews"
	"github.com/akhdanfadh/hnkeep/internal/harmonic"
)

type Config struct {
	InputPath  string
	OutputPath string
}

// parseFlags parses command-line flags and returns a Config struct.
func parseFlags() (*Config, error) {
	// NOTE: go flag package does not support alias natively.
	// - https://github.com/golang/go/issues/35761
	inputPath := flag.String("input", "", "Input file path (Harmonic TXT export). Default to stdin.")
	flag.StringVar(inputPath, "i", "", "alias for -input")
	outputPath := flag.String("output", "", "Output file path (Karakeep JSON import). Default to stdout.")
	flag.StringVar(outputPath, "o", "", "alias for -output")

	flag.Parse()
	return &Config{
		InputPath:  *inputPath,
		OutputPath: *outputPath,
	}, nil
}

// readInput reads the input from the specified path or stdin if the path is empty.
func readInput(path string) (string, error) {
	var r io.Reader = os.Stdin // fallback
	if path != "" {
		f, err := os.Open(path)
		if err != nil {
			return "", err
		}
		defer func() { _ = f.Close() }() // ignore error, less critical for read
		r = f
	}

	data, err := io.ReadAll(r)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// writeOutput writes the output to the specified path or stdout if the path is empty.
func writeOutput(path, data string) (err error) {
	// NOTE: Use bufio.Writer here if you are making many small writes and want to avoid
	// overhead of frequent syscalls. However, we are writing only once in this code.
	// - https://pkg.go.dev/bufio#Writer
	var w io.Writer = os.Stdout // fallback
	if path != "" {
		// NOTE: I wrote a bug here by using `err :=` which shadowed the named return
		// `err` the defer needs to report Close() errors. So be careful with that.
		f, createErr := os.Create(path)
		if createErr != nil {
			return createErr
		}
		defer func() {
			if closeErr := f.Close(); closeErr != nil && err == nil {
				err = closeErr
			}
		}()
		w = f
	}

	_, err = io.WriteString(w, data)
	return err
}

// Run executes the CLI with the provided arguments.
func Run() error {
	cfg, err := parseFlags()
	if err != nil {
		return err
	}

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

	input, err := readInput(cfg.InputPath)
	if err != nil {
		return fmt.Errorf("reading input: %w", err)
	}

	bookmarks, err := harmonic.Parse(input)
	if err != nil {
		return fmt.Errorf("parsing input: %w", err)
	}

	// TODO: do something
	_ = hackernews.NewClient()

	// NOTE: Go strings are immutable, so using string concatenation in a loop
	// can lead to excessive memory allocations (a hint from `go vet`).
	// - https://stackoverflow.com/questions/1760757/how-to-efficiently-concatenate-strings-in-go/47798475#47798475.
	var output strings.Builder
	for _, bm := range bookmarks {
		fmt.Fprintf(&output, "%d %d\n", bm.ID, bm.Timestamp)
	}
	if err := writeOutput(cfg.OutputPath, output.String()); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	return nil
}
