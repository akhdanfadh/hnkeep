package converter

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/akhdanfadh/hnkeep/internal/hackernews"
	"github.com/akhdanfadh/hnkeep/internal/harmonic"
	"github.com/akhdanfadh/hnkeep/internal/logger"
)

// Options represents additional options for the conversion process.
type Options struct {
	Tags         []string // Tags to apply to all bookmarks
	NoteTemplate string   // Template for note field (empty = no note)
	Dedupe       bool     // Merge duplicate URLs, combining their notes
}

// noteSeparator is used to join notes when merging duplicate URLs.
const noteSeparator = "\n\n---\n\n"

// ItemFetcher defines the interface for fetching Hacker News items.
type ItemFetcher interface {
	GetItem(ctx context.Context, id int) (*hackernews.Item, error)
}

const defaultConcurrency = 5

// getDefaultFetcher returns the default Hacker News client (item fetcher).
func getDefaultFetcher() ItemFetcher {
	return hackernews.NewClient()
}

// Converter represents the conversion pipeline orchestrator.
type Converter struct {
	fetcher     ItemFetcher
	concurrency int
	logger      logger.Logger
}

// Option configures the Converter.
type Option func(*Converter)

// New creates a new Converter with the given fetcher and options.
func New(opts ...Option) *Converter {
	c := &Converter{
		fetcher:     getDefaultFetcher(),
		concurrency: defaultConcurrency,
		logger:      logger.Noop(),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// WithFetcher sets a custom ItemFetcher for the Converter.
func WithFetcher(fetcher ItemFetcher) Option {
	return func(c *Converter) {
		c.fetcher = fetcher
	}
}

// WithConcurrency sets the number of parallel HN fetches.
func WithConcurrency(n int) Option {
	return func(c *Converter) {
		c.concurrency = n
	}
}

// WithLogger sets the logger for info/warn/error messages.
func WithLogger(l logger.Logger) Option {
	return func(c *Converter) {
		c.logger = l
	}
}

// FetchItems fetches Hacker News items for the given bookmarks concurrently.
func (c *Converter) FetchItems(ctx context.Context, bookmarks []harmonic.Bookmark) (map[int]*hackernews.Item, error) {
	type result struct {
		bookmark harmonic.Bookmark
		item     *hackernews.Item
		err      error
	}
	results := make(chan result, len(bookmarks))
	semaphore := make(chan struct{}, c.concurrency)

	total := len(bookmarks)
	var counter atomic.Int32 // for logging progress

	// fetch items with semaphore
	var wg sync.WaitGroup
	for _, bm := range bookmarks {
		wg.Add(1)
		go func(bookmark harmonic.Bookmark) { // pass bm as param to avoid closure capture
			defer wg.Done()

			// check for cancellation before acquiring
			// this prevents queued goroutines from starting new work after Ctrl+C
			select {
			case <-ctx.Done():
				return
			case semaphore <- struct{}{}: // acquire
			}
			defer func() { <-semaphore }() // release

			// check again after acquiring (in case cancelled while waiting)
			if ctx.Err() != nil {
				return
			}

			item, err := c.fetcher.GetItem(ctx, bookmark.ID)
			// don't send result (avoid blocking on full channel)
			if ctx.Err() != nil {
				return
			}

			n := counter.Add(1)
			c.logger.Info("fetched %d/%d (ID: %d)", n, total, bookmark.ID)
			results <- result{bookmark: bookmark, item: item, err: err}
		}(bm)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	// process fetch results
	items := make(map[int]*hackernews.Item)
	for r := range results {
		// check for cancellation while processing results
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if r.err != nil {
			if errors.Is(r.err, hackernews.ErrItemNotFound) {
				c.logger.Warn("item %d not found, skipping", r.bookmark.ID)
			} else {
				c.logger.Warn("failed to fetch item %d: %v, skipping", r.bookmark.ID, r.err)
			}
			continue
		}
		items[r.bookmark.ID] = r.item
	}

	return items, nil
}

// Convert converts the fetched items and bookmarks into Karakeep export format.
// Returns the export and the number of duplicate URLs that were merged.
func (c *Converter) Convert(bookmarks []harmonic.Bookmark, items map[int]*hackernews.Item, opts Options) (Schema, int) {
	var export Schema
	seenURLs := make(map[string]int) // url -> index in export.Bookmarks
	dedupedCount := 0

	for _, bm := range bookmarks {
		item, ok := items[bm.ID]
		if !ok {
			continue // skip missing items (deleted or fetch error)
		}

		// resolve url
		var url string
		if item.URL != "" {
			url = item.URL
		} else {
			url = hackernews.DiscussionURL(item.ID)
		}

		// render note template
		var note string
		if opts.NoteTemplate != "" {
			smartURL := hackernews.DiscussionURL(item.ID)
			if item.URL == "" {
				smartURL = ""
			}
			note = strings.NewReplacer(
				"{{smart_url}}", smartURL,
				"{{item_url}}", item.URL,
				"{{hn_url}}", hackernews.DiscussionURL(item.ID),
				"{{id}}", strconv.Itoa(item.ID),
				"{{title}}", item.Title,
				"{{author}}", item.By,
				"{{date}}", time.Unix(item.Time, 0).Format("2006-01-02"),
			).Replace(opts.NoteTemplate)
		}

		// check for duplicate URL
		if opts.Dedupe {
			if idx, exists := seenURLs[url]; exists {
				// merge notes with separator
				if note != "" {
					existing := export.Bookmarks[idx]
					if existing.Note != nil && *existing.Note != "" {
						merged := *existing.Note + noteSeparator + note
						export.Bookmarks[idx].Note = &merged
					} else {
						export.Bookmarks[idx].Note = &note
					}
				}
				dedupedCount++
				continue // skip adding new bookmark
			}
		}

		// build struct
		kb := Bookmark{
			CreatedAt: bm.Timestamp,
			Title:     &item.Title,
			Content:   NewBookmarkContent(url),
			Tags:      opts.Tags,
		}

		if note != "" { // avoid empty rendered note
			kb.Note = &note
		}

		if opts.Dedupe {
			seenURLs[url] = len(export.Bookmarks) // record index
		}
		export.Bookmarks = append(export.Bookmarks, kb)
	}

	return export, dedupedCount
}
