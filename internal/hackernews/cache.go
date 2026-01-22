package hackernews

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/akhdanfadh/hnkeep/internal/logger"
)

// Cache permanent-error states for negative caching.
const (
	cacheErrDeleted = "deleted"
	cacheErrDead    = "dead"
)

// cacheEntry wraps an item with optional error state for negative caching.
type cacheEntry struct {
	Item  *Item  `json:"item,omitempty"`
	Error string `json:"error,omitempty"`
}

// inflightCall deduplicates concurrent fetches for the same item (singleflight pattern).
type inflightCall struct {
	wg   sync.WaitGroup
	item *Item
	err  error
}

// CachedClient wraps a Client with caching capabilities.
type CachedClient struct {
	client   *Client
	cacheDir string
	logger   logger.Logger

	mu        sync.Mutex
	inflight  map[int]*inflightCall
	cacheHits atomic.Int32
}

// CacheOption configures the CachedClient.
type CacheOption func(*CachedClient)

// WithCacheLogger sets a custom Logger for the CachedClient.
func WithCacheLogger(l logger.Logger) CacheOption {
	return func(c *CachedClient) {
		c.logger = l
	}
}

// NewCachedClient creates a client that caches responses in the given directory.
func NewCachedClient(client *Client, cacheDir string, opts ...CacheOption) (*CachedClient, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, err
	}
	c := &CachedClient{
		client:   client,
		cacheDir: cacheDir,
		logger:   logger.Noop(),
		inflight: make(map[int]*inflightCall),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// GetItem retrieves an item by ID, using the cache if available.
func (c *CachedClient) GetItem(ctx context.Context, id int) (*Item, error) {
	// check for early cancellation
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	// try read from cache (includes negative cache hits)
	item, err := c.readCache(id)
	if err == nil {
		c.cacheHits.Add(1)
		c.logger.Info("cache hit for item %d", id)
		return item, nil
	}
	if errors.Is(err, ErrItemDeleted) || errors.Is(err, ErrItemDead) {
		c.cacheHits.Add(1)
		c.logger.Info("cache hit for item %d (negative)", id)
		return nil, err // cached error state
	}

	// cache miss, try to deduplicate concurrent fetches
	c.mu.Lock()
	if call, ok := c.inflight[id]; ok {
		// another goroutine is already fetching this item, wait for it
		c.mu.Unlock()
		call.wg.Wait() // block until fetch is done
		return call.item, call.err
	}

	// otherwise, we are the first so create an inflightCall
	call := &inflightCall{}
	call.wg.Add(1)
	c.inflight[id] = call
	c.mu.Unlock()

	// fetch from API and cache result (best-effort), outside lock
	call.item, call.err = c.client.GetItem(ctx, id)
	if ctx.Err() == nil { // don't cache incomplete results
		_ = c.writeCache(id, call.item, call.err)
	}

	// signal waiting goroutines and cleanup
	c.mu.Lock()
	delete(c.inflight, id)
	c.mu.Unlock()
	call.wg.Done()

	return call.item, call.err
}

// CacheHits returns the number of cache hits (both positive and negative).
func (c *CachedClient) CacheHits() int {
	return int(c.cacheHits.Load())
}

// getCachePath returns the file path for the cached item with the given ID.
func (c *CachedClient) getCachePath(id int) string {
	return filepath.Join(c.cacheDir, fmt.Sprintf("%d.json", id))
}

// writeCache writes an item or error state to the cache.
// Caches the item on success, or the error state for permanent errors (deleted/dead).
func (c *CachedClient) writeCache(id int, item *Item, err error) error {
	var entry cacheEntry

	switch {
	case err == nil && item != nil:
		entry.Item = item
	case errors.Is(err, ErrItemDeleted):
		entry.Error = cacheErrDeleted
	case errors.Is(err, ErrItemDead):
		entry.Error = cacheErrDead
	default:
		return nil // don't cache unknown errors or nil results
	}

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	return os.WriteFile(c.getCachePath(id), data, 0o644)
}

// ClearCache removes all cached items.
func (c *CachedClient) ClearCache() error {
	if err := os.RemoveAll(c.cacheDir); err != nil {
		return err
	}
	// recreate cacheDir so subsequent writeCache calls don't fail
	return os.MkdirAll(c.cacheDir, 0o755)
}

// readCache reads the item with the given ID from the cache.
// Returns the cached error if a negative cache entry exists.
func (c *CachedClient) readCache(id int) (*Item, error) {
	data, err := os.ReadFile(c.getCachePath(id))
	if err != nil {
		return nil, err
	}

	var entry cacheEntry
	if err := json.Unmarshal(data, &entry); err != nil {
		return nil, err
	}

	// both fields set is invalid as per the writeCache logic
	if entry.Item != nil && entry.Error != "" {
		return nil, os.ErrNotExist
	}

	// check for cached error state
	if entry.Error != "" {
		switch entry.Error {
		case cacheErrDeleted:
			return nil, ErrItemDeleted
		case cacheErrDead:
			return nil, ErrItemDead
			// default: ignore unknown error states
		}
	}

	// handle invalid/corrupted cache entries
	// otherwise returning (nil, nil) would cause nil pointer dereference
	if entry.Item == nil {
		return nil, os.ErrNotExist
	}

	return entry.Item, nil
}
