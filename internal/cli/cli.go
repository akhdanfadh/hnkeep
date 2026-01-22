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
	if cfg.InputPath == "" && logger.IsTTY(os.Stdin) {
		flag.Usage()
		return nil
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

	// early exit if no bookmarks to process
	if stats.afterLimit == 0 {
		fmt.Fprintf(os.Stderr, "Warning: no bookmarks to process (found %d, all filtered out)\n", stats.found)
		return nil
	}

	// pre-flight connectivity check for sync mode (includes dry-run)
	var karakeepClient *karakeep.Client
	if cfg.Sync {
		karakeepClient = karakeep.NewClient(cfg.APIBaseURL, cfg.APIKey,
			karakeep.WithTimeout(cfg.APITimeout),
		)

		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "Checking Karakeep API connectivity... ")
		}
		if err := karakeepClient.CheckConnectivity(ctx); err != nil {
			if cfg.Verbose {
				fmt.Fprintf(os.Stderr, "failed\n")
			}
			return fmt.Errorf("karakeep API check failed: %w", err)
		}
		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "ok\n")
		}
	}

	// dry run mode: give stats on the input and exit
	if cfg.DryRun {
		printDryRunMode(stats, bookmarks, cfg.Sync)
		return nil
	}

	// configure logger and clients
	log := logger.NewStdLogger(os.Stderr, !cfg.Verbose)
	client := hackernews.NewClient(hackernews.WithLogger(log))
	var fetcher converter.ItemFetcher = client

	// use cached client if cache dir is set
	if cfg.CacheDir != "" {
		cachedClient, err := hackernews.NewCachedClient(client, cfg.CacheDir, hackernews.WithCacheLogger(log))
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

	// setup progress indicator if stderr is a TTY and not verbose (verbose has its own logging)
	var progressFetch *logger.TTYProgresser
	if !cfg.Verbose && logger.IsStderrTTY() {
		progressFetch = logger.NewProgresser(os.Stderr, "Fetching: %d/%d")
	}

	// perform conversion
	convOpts := []converter.Option{
		converter.WithFetcher(fetcher),
		converter.WithConcurrency(cfg.Concurrency),
		converter.WithLogger(log),
	}
	if progressFetch != nil {
		convOpts = append(convOpts, converter.WithProgress(progressFetch))
	}
	conv := converter.New(convOpts...)

	stats.fetchStart = time.Now()
	items, err := conv.FetchItems(ctx, bookmarks)
	stats.fetchEnd = time.Now()
	if progressFetch != nil {
		progressFetch.Clear()
	}
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

		// setup progress indicator for sync (same condition as fetch)
		var progressSync *logger.TTYProgresser
		if !cfg.Verbose && logger.IsStderrTTY() {
			progressSync = logger.NewProgresser(os.Stderr, "Syncing: %d/%d")
		}

		// add logger to the existing client (created during connectivity check)
		karakeepClient = karakeep.NewClient(cfg.APIBaseURL, cfg.APIKey,
			karakeep.WithTimeout(cfg.APITimeout),
			karakeep.WithLogger(log),
		)

		// pre-fetch existing bookmarks for client-side deduplication
		var existingBookmarks map[string]karakeep.ExistingBookmark
		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "Pre-fetching existing bookmarks... ")
		}
		existingBookmarks, err = karakeepClient.ListBookmarks(ctx)
		if err != nil {
			if cfg.Verbose {
				fmt.Fprintf(os.Stderr, "failed\n")
			}
			return fmt.Errorf("pre-fetching bookmarks: %w", err)
		}
		stats.prefetched = len(existingBookmarks)
		if cfg.Verbose {
			fmt.Fprintf(os.Stderr, "found %d\n", stats.prefetched)
		}

		syncOpts := []syncer.Option{
			syncer.WithConcurrency(cfg.Concurrency),
			syncer.WithLogger(log),
		}
		if progressSync != nil {
			syncOpts = append(syncOpts, syncer.WithProgress(progressSync))
		}
		sync := syncer.New(karakeepClient, syncOpts...)

		stats.syncStart = time.Now()
		syncStatus := sync.Sync(ctx, export.Bookmarks)
		stats.syncEnd = time.Now()
		if progressSync != nil {
			progressSync.Clear()
		}

		stats.syncCreated = syncStatus[syncer.SyncCreated]
		stats.syncUpdated = syncStatus[syncer.SyncUpdated]
		stats.syncSkipped = syncStatus[syncer.SyncSkipped]
		stats.syncFailed = syncStatus[syncer.SyncFailed]

		printSyncSummary(stats)

		// return error for non-zero exit code (details already logged inline)
		if stats.syncFailed > 0 {
			return fmt.Errorf("%d bookmark(s) failed to sync", stats.syncFailed)
		}

		return nil
	}

	// default mode: write to file/stdout
	if err := writeOutput(cfg.OutputPath, export); err != nil {
		return fmt.Errorf("writing output: %w", err)
	}

	printSummary(stats)
	return nil
}
