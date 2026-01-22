package syncer

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/akhdanfadh/hnkeep/internal/converter"
	"github.com/akhdanfadh/hnkeep/internal/karakeep"
	"github.com/akhdanfadh/hnkeep/internal/logger"
)

// noteSeparator is used to join notes when merging with existing Karakeep notes.
const (
	noteSeparator      = "\n\n---\n\n"
	defaultConcurrency = 5
)

// Syncer represents the syncer pipeline orchestrator.
type Syncer struct {
	client      *karakeep.Client
	concurrency int
	logger      logger.Logger
	progresser  logger.Progresser
}

// Option configures the Syncer.
type Option func(s *Syncer)

// New creates a new Syncer with the given client and options.
func New(client *karakeep.Client, opts ...Option) *Syncer {
	s := &Syncer{
		client:      client,
		concurrency: defaultConcurrency,
		logger:      logger.Noop(),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// WithConcurrency sets the number of parallel API fetches.
func WithConcurrency(n int) Option {
	return func(c *Syncer) {
		c.concurrency = n
	}
}

// WithLogger sets the logger for info/warn/error messages.
func WithLogger(l logger.Logger) Option {
	return func(c *Syncer) {
		c.logger = l
	}
}

// WithProgress sets a progresser for progress updates during sync.
func WithProgress(p logger.Progresser) Option {
	return func(c *Syncer) {
		c.progresser = p
	}
}

// SyncStatus represents the result of a sync operation.
type SyncStatus int

const (
	SyncFailed SyncStatus = iota
	SyncCreated
	SyncUpdated
	SyncSkipped
)

// SyncError represents an error that occurred during syncing a bookmark.
type SyncError struct {
	URL string
	Err error
}

// Error implements the error interface for SyncError.
func (e SyncError) Error() string {
	return fmt.Sprintf("syncing bookmark %q: %v", e.URL, e.Err)
}

// Unwrap returns the underlying error for use with errors.Is and errors.As.
// Actually HTTPError doesn't need Unwrap as it is not wrapping another error,
// and just has StatusCode and Body fields, but just in case in the future?
func (e SyncError) Unwrap() error {
	return e.Err
}

// Sync synchronizes the given converted bookmarks to Karakeep.
func (s *Syncer) Sync(ctx context.Context, bookmarks []converter.Bookmark) (map[SyncStatus]int, []SyncError) {
	type syncTaskResult struct {
		url    string
		status SyncStatus
		err    error
	}
	syncTaskCh := make(chan syncTaskResult, len(bookmarks))
	semaphoreCh := make(chan struct{}, s.concurrency)

	total := len(bookmarks)
	var counter atomic.Int32 // for logging progress

	// sync bookmarks with semaphore
	var wg sync.WaitGroup
	for _, bm := range bookmarks {
		wg.Add(1)
		go func(bookmark converter.Bookmark) {
			defer wg.Done()

			// check for cancellation before acquiring
			select {
			case <-ctx.Done():
				return
			case semaphoreCh <- struct{}{}: // acquire
			}
			defer func() { <-semaphoreCh }() // release

			// check again after acquiring (in case cancelled while waiting)
			if ctx.Err() != nil {
				return
			}

			status, err := s.syncTask(ctx, bookmark)
			// skip sending result after cancellation
			if ctx.Err() != nil {
				return
			}

			n := counter.Add(1)
			if s.progresser != nil {
				s.progresser.Update(int(n), total)
			}
			s.logger.Info("pushed %d/%d", n, total)
			syncTaskCh <- syncTaskResult{url: bookmark.Content.URL, status: status, err: err}
		}(bm)
	}

	go func() {
		wg.Wait()
		close(syncTaskCh)
	}()

	// process sync results
	status := make(map[SyncStatus]int)
	var errs []SyncError
	for r := range syncTaskCh {
		switch r.status {
		case SyncFailed:
			status[SyncFailed]++
			errs = append(errs, SyncError{URL: r.url, Err: r.err})
			s.logger.Warn("failed to push %s: %v", r.url, r.err)
		case SyncCreated:
			status[SyncCreated]++
		case SyncUpdated:
			status[SyncUpdated]++
		case SyncSkipped:
			status[SyncSkipped]++
		}

		// check for cancellation after processing
		if ctx.Err() != nil {
			return status, errs
		}
	}
	return status, errs
}

// syncTask performs the sync operation for a single bookmark.
//
// The following business logic is made:
//  1. Create the bookmark (or get existing) by passing url, createdAt, title, and note.
//  2. Since attaching tags is idempotent, always attach tags if converted has any.
//  3. If it is newly created, we're done.
//  4. If the (unedited) existing is returned, we check whether to update createdAt (by earliest) and/or note (see mergeNotes).
func (s *Syncer) syncTask(ctx context.Context, convertedBM converter.Bookmark) (SyncStatus, error) {
	// create or get existing bookmark
	karakeepBM, alreadyExists, err := s.client.CreateBookmark(ctx,
		convertedBM.Content.URL,
		unixToISO8601(convertedBM.CreatedAt),
		convertedBM.Title,
		convertedBM.Note,
	)
	if err != nil {
		return SyncFailed, fmt.Errorf("creating bookmark: %w", err)
	}

	// attach tags if any
	if len(convertedBM.Tags) > 0 {
		if err := s.client.AttachTags(ctx, karakeepBM.ID, convertedBM.Tags); err != nil {
			return SyncFailed, fmt.Errorf("attaching tags: %w", err)
		}
	}

	if !alreadyExists {
		return SyncCreated, nil
	}

	// handle timestamp update: use the earlier
	var updatedCreatedAt *string
	var timestampChanged bool
	karakeepCreatedAtUnix, err := iso8601ToUnix(karakeepBM.CreatedAt)
	if err != nil {
		return SyncFailed, fmt.Errorf("parsing existing createdAt: %w", err)
	}
	if convertedBM.CreatedAt < karakeepCreatedAtUnix {
		earlierCreatedAt := unixToISO8601(convertedBM.CreatedAt)
		updatedCreatedAt = &earlierCreatedAt
		timestampChanged = true
	}

	// handle note update: merge if needed
	updatedNote, noteChanged := mergeNotes(karakeepBM.Note, convertedBM.Note)

	// decide update or skip
	if !timestampChanged && !noteChanged {
		return SyncSkipped, nil
	}
	if err := s.client.UpdateBookmark(ctx, karakeepBM.ID, updatedCreatedAt, updatedNote); err != nil {
		return SyncFailed, fmt.Errorf("updating bookmark: %w", err)
	}
	return SyncUpdated, nil
}

// mergeNotes merges a new note into an existing note.
// Returns the merged note and whether an update is needed.
//
// Update logic:
//   - If the incoming note is nil or empty, no update is needed.
//   - If the existing note already contains the incoming note, skip (idempotent).
//   - If the existing note is empty, use the incoming note directly.
//   - If the existing note is non-empty, append with noteSeparator.
func mergeNotes(existing, incoming *string) (merged *string, needsUpdate bool) {
	existingNote := ""
	if existing != nil {
		existingNote = *existing
	}

	if incoming == nil || *incoming == "" { // nil check before dereference
		return existing, false
	}

	if strings.Contains(existingNote, *incoming) { // idempotency here
		return existing, false
	}

	if existingNote == "" {
		result := strings.TrimSpace(*incoming)
		if result == "" {
			return nil, false
		}
		return &result, true
	}

	result := strings.TrimSpace(existingNote + noteSeparator + *incoming)
	return &result, true
}

// unixToISO8601 converts a Unix timestamp (in seconds) to an ISO8601 date string.
func unixToISO8601(ts int64) string {
	return time.Unix(ts, 0).Format(time.RFC3339)
}

// iso8601ToUnix converts an ISO8601 date string to a Unix timestamp (in seconds).
func iso8601ToUnix(iso string) (int64, error) {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return 0, fmt.Errorf("parsing ISO8601 date %q: %w", iso, err)
	}
	return t.Unix(), nil
}
