package cli

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/akhdanfadh/hnkeep/internal/converter"
	"github.com/akhdanfadh/hnkeep/internal/hackernews"
	"github.com/akhdanfadh/hnkeep/internal/harmonic"
	"github.com/akhdanfadh/hnkeep/internal/karakeep"
)

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
func writeOutput(path string, export karakeep.Export) (err error) {
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

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ") // pretty print
	return encoder.Encode(export)
}

// filterByDate filters bookmarks by before and after timestamps.
func filterByDate(bookmarks []harmonic.Bookmark, before, after int64) []harmonic.Bookmark {
	if after == 0 && before == 0 {
		return bookmarks
	} // basic validation

	filtered := make([]harmonic.Bookmark, 0, len(bookmarks))
	for _, bm := range bookmarks {
		if after > 0 && bm.Timestamp < after {
			continue
		}
		if before > 0 && bm.Timestamp > before {
			continue
		}
		filtered = append(filtered, bm)
	}
	return filtered
}

// Run executes the CLI with the provided arguments.
func Run() error {
	cfg, err := parseFlags()
	if err != nil {
		return fmt.Errorf("parsing flags: %w", err)
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

	// parse harmonic export
	bookmarks, err := harmonic.Parse(input)
	if err != nil {
		return fmt.Errorf("parsing input: %w", err)
	}

	// apply filters
	if cfg.Before > 0 || cfg.After > 0 {
		bookmarks = filterByDate(bookmarks, cfg.Before, cfg.After)
	}
	if cfg.Limit > 0 && cfg.Limit < len(bookmarks) {
		bookmarks = bookmarks[:cfg.Limit]
	}

	// dry run mode: give stats on the input and exit
	if cfg.DryRun {
		printDryRunMode(bookmarks)
		return nil
	}

	// configure logger and clients
	logger := NewLogger(os.Stderr, cfg.Quiet)
	client := hackernews.NewClient()
	var fetcher converter.ItemFetcher = client

	// use cached client if cache dir is set
	if cfg.CacheDir != "" {
		cachedClient, err := hackernews.NewCachedClient(client, cfg.CacheDir, hackernews.WithLogger(logger))
		if err != nil {
			return fmt.Errorf("creating cached client: %w", err)
		}
		if cfg.ClearCache {
			if err := cachedClient.ClearCache(); err != nil {
				return fmt.Errorf("clearing cache: %w", err)
			}
		}
		fetcher = cachedClient
	}

	// perform conversion
	conv := converter.New(
		converter.WithFetcher(fetcher),
		converter.WithConcurrency(cfg.Concurrency),
		converter.WithLogger(logger),
	)
	items := conv.FetchItems(bookmarks)
	export := conv.Convert(bookmarks, items, converter.Options{
		Tags:         cfg.Tags,
		NoteTemplate: cfg.NoteTemplate,
	})

	if err := writeOutput(cfg.OutputPath, export); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	return nil
}

// printDryRunMode prints statistics about the bookmarks without making any API calls.
func printDryRunMode(bookmarks []harmonic.Bookmark) {
	fmt.Fprintf(os.Stderr, "=== Dry Run ===\n")
	fmt.Fprintf(os.Stderr, "Bookmarks\t: %d\n", len(bookmarks))
	if len(bookmarks) == 0 {
		return
	}

	// find date and id range
	minTS, maxTS := bookmarks[0].Timestamp, bookmarks[0].Timestamp
	minID, maxID := bookmarks[0].ID, bookmarks[0].ID
	for _, b := range bookmarks {
		if b.Timestamp < minTS {
			minTS = b.Timestamp
		}
		if b.Timestamp > maxTS {
			maxTS = b.Timestamp
		}
		if b.ID < minID {
			minID = b.ID
		}
		if b.ID > maxID {
			maxID = b.ID
		}
	}
	oldest := time.Unix(minTS, 0).UTC().Format("2006-01-02 15:04:05")
	newest := time.Unix(maxTS, 0).UTC().Format("2006-01-02 15:04:05")

	fmt.Fprintf(os.Stderr, "Earliest entry\t: %d at %s UTC\n", minID, oldest)
	fmt.Fprintf(os.Stderr, "Latest entry\t: %d at %s UTC\n", maxID, newest)
	fmt.Fprintf(os.Stderr, "\nNo API calls made.\n")
}
