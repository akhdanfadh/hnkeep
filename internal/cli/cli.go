package cli

import (
	"context"
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
	"github.com/akhdanfadh/hnkeep/internal/logger"
	"github.com/akhdanfadh/hnkeep/internal/syncer"
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
func writeOutput(path string, export converter.Schema) (err error) {
	var w io.Writer = os.Stdout // fallback
	if path != "" {
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

// Run executes the CLI with the provided CLI arguments.
func Run(ctx context.Context) error {
	var stats stats
	stats.totalStart = time.Now()

	cfg, err := parseFlags()
	if err != nil {
		return fmt.Errorf("parsing flags: %w", err)
	}

	// if no input data is given and stdin is a terminal, show usage and exit
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
	stats.found = len(bookmarks)

	// apply filters
	if cfg.Before > 0 || cfg.After > 0 {
		bookmarks = filterByDate(bookmarks, cfg.Before, cfg.After)
	}
	stats.afterFilter = len(bookmarks)
	if cfg.Limit > 0 && cfg.Limit < len(bookmarks) {
		bookmarks = bookmarks[:cfg.Limit]
	}
	stats.afterLimit = len(bookmarks)

	// dry run mode: give stats on the input and exit
	if cfg.DryRun {
		printDryRunMode(stats, bookmarks)
		return nil
	}

	// configure logger and clients
	logger := logger.NewStdLogger(os.Stderr, cfg.Quiet)
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

	stats.fetchStart = time.Now()
	items, err := conv.FetchItems(ctx, bookmarks)
	stats.fetchEnd = time.Now()
	if err != nil {
		return fmt.Errorf("fetching items: %w", err)
	}
	stats.skipped = stats.afterLimit - len(items)

	if cc, ok := fetcher.(*hackernews.CachedClient); ok {
		stats.cacheHits = cc.CacheHits()
	}

	export, dedupedCount := conv.Convert(bookmarks, items, converter.Options{
		Tags:         cfg.Tags,
		NoteTemplate: cfg.NoteTemplate,
		Dedupe:       cfg.Dedupe,
	})
	stats.deduped = dedupedCount
	stats.converted = len(export.Bookmarks)

	// sync mode: push directly to Karakeep API
	if cfg.Sync {
		if cfg.OutputPath != "" {
			fmt.Fprintf(os.Stderr, "Warning: --output is ignored in sync mode\n")
		}

		karakeepClient := karakeep.NewClient(cfg.APIBaseURL, cfg.APIKey)
		sync := syncer.New(
			karakeepClient,
			syncer.WithConcurrency(cfg.Concurrency),
			syncer.WithLogger(logger),
		)

		stats.syncStart = time.Now()
		syncStatus, syncErrs := sync.Sync(ctx, export.Bookmarks)
		stats.syncEnd = time.Now()

		stats.syncCreated = syncStatus[syncer.SyncCreated]
		stats.syncUpdated = syncStatus[syncer.SyncUpdated]
		stats.syncSkipped = syncStatus[syncer.SyncSkipped]
		stats.syncFailed = syncStatus[syncer.SyncFailed]

		if !cfg.Quiet {
			printSyncSummary(stats)
		}

		// print sync errors
		if len(syncErrs) > 0 {
			fmt.Fprintf(os.Stderr, "\nSync errors:\n")
			for _, e := range syncErrs {
				fmt.Fprintf(os.Stderr, "  - %s: %v\n", e.URL, e.Err)
			}
		}

		return nil
	}

	// default mode: write to file/stdout
	if err := writeOutput(cfg.OutputPath, export); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	if !cfg.Quiet {
		printSummary(stats)
	}
	return nil
}
