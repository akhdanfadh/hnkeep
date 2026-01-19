package converter

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/akhdanfadh/hnkeep/internal/hackernews"
	"github.com/akhdanfadh/hnkeep/internal/harmonic"
	"github.com/akhdanfadh/hnkeep/internal/karakeep"
)

// Options represents additional options for the conversion process.
type Options struct {
	Tags         []string // Tags to apply to all bookmarks
	NoteTemplate string   // Template for note field (empty = no note)
}

// ItemFetcher defines the interface for fetching Hacker News items.
type ItemFetcher interface {
	GetItem(id int) (*hackernews.Item, error)
}

// NOTE: Go does not support constant arrays, maps, or slices.
// - https://blog.boot.dev/golang/golang-constant-maps-slices
// - https://stackoverflow.com/questions/13137463/declare-a-constant-array

const defaultConcurrency = 5

// getDefaultFetcher returns the default Hacker News client (item fetcher).
func getDefaultFetcher() ItemFetcher {
	return hackernews.NewClient()
}

// getDefaultOutput returns the default output file (stderr).
func getDefaultOutput() io.Writer {
	return os.Stderr
}

// Converter represents the conversion pipeline orchestrator.
type Converter struct {
	fetcher     ItemFetcher
	concurrency int
	output      io.Writer // for status/warning messages
}

// Option configures the Converter.
type Option func(*Converter)

// New creates a new Converter with the given fetcher and options.
func New(opts ...Option) *Converter {
	c := &Converter{
		fetcher:     getDefaultFetcher(),
		concurrency: defaultConcurrency,
		output:      getDefaultOutput(),
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

// WithOutput sets the output writer for status/warning messages.
func WithOutput(w io.Writer) Option {
	return func(c *Converter) {
		c.output = w
	}
}

// FetchItems fetches Hacker News items for the given bookmarks concurrently.
func (c *Converter) FetchItems(bookmarks []harmonic.Bookmark) map[int]*hackernews.Item {
	type result struct {
		bookmark harmonic.Bookmark
		item     *hackernews.Item
		err      error
	}
	results := make(chan result, len(bookmarks))
	semaphore := make(chan struct{}, c.concurrency)

	// fetch items with semaphore
	// NOTE: Having read "Grokking Concurrency" really helped me understand this concurrency pattern.
	var wg sync.WaitGroup
	for _, bm := range bookmarks {
		wg.Add(1)
		// NOTE: We need to pass bm as parameter to avoid closure capture issue.
		// Otherwise, all goroutines would capture the same loop variable reference (last value in loop).
		// - https://go.dev/wiki/CommonMistakes
		// - https://go.dev/doc/faq#closures_and_goroutines
		go func(bookmark harmonic.Bookmark) {
			defer wg.Done()
			semaphore <- struct{}{}        // acquire
			defer func() { <-semaphore }() // release

			item, err := c.fetcher.GetItem(bookmark.ID)
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
		if r.err != nil {
			if errors.Is(r.err, hackernews.ErrItemNotFound) {
				// Fprintf error itself is non-critical, ignore
				_, _ = fmt.Fprintf(c.output, "warning: item %d not found, skipping\n", r.bookmark.ID)
			} else {
				_, _ = fmt.Fprintf(c.output, "warning: failed to fetch item %d: %v, skipping\n", r.bookmark.ID, r.err)
			}
			continue
		}
		items[r.bookmark.ID] = r.item
	}

	return items
}

// Convert converts the fetched items and bookmarks into Karakeep export format.
func (c *Converter) Convert(bookmarks []harmonic.Bookmark, items map[int]*hackernews.Item, opts Options) karakeep.Export {
	var export karakeep.Export
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

		// build struct
		kb := karakeep.Bookmark{
			CreatedAt: bm.Timestamp / 1000, // convert ms to s
			Title:     &item.Title,
			Content: &karakeep.BookmarkContent{
				Link: &karakeep.LinkContent{
					Type: karakeep.BookmarkTypeLink,
					URL:  url,
				},
			},
			Tags: opts.Tags,
		}

		// render note template
		if opts.NoteTemplate != "" {
			smartURL := hackernews.DiscussionURL(item.ID)
			if item.URL == "" {
				smartURL = ""
			}

			note := strings.NewReplacer(
				"{{smart_url}}", smartURL,
				"{{item_url}}", item.URL,
				"{{hn_url}}", hackernews.DiscussionURL(item.ID),
				"{{id}}", strconv.Itoa(item.ID),
				"{{title}}", item.Title,
				"{{author}}", item.By,
				"{{date}}", time.Unix(item.Time, 0).Format("2006-01-02"),
			).Replace(opts.NoteTemplate)

			if note != "" { // avoid empty rendered note
				kb.Note = &note
			}
		}

		export.Bookmarks = append(export.Bookmarks, kb)
	}
	return export
}
