package hackernews

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
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

// Logger defines the interface for logging messages.
type Logger interface {
	Info(format string, args ...any)
	Warn(format string, args ...any)
	Error(format string, args ...any)
}

// noopLogger is a Logger implementation that does nothing.
type noopLogger struct{}

func (noopLogger) Info(string, ...any)  {}
func (noopLogger) Warn(string, ...any)  {}
func (noopLogger) Error(string, ...any) {}

// NOTE: This is a simplified "singleflight" concurrency control implementation.
// It deduplicates concurrent requests for the same key (item ID in our case)
// so only one fetch happens while others wait for the result.
// If not configured, multiple goroutines requesting the same item ID could all
// miss cache, all fetch from the API, and all write to the same file concurrently.
// - https://pkg.go.dev/golang.org/x/sync/singleflight

// inflightCall represents an in-progress fetch for an item.
// Multiple goroutines requesting the same item ID share one inflightCall.
type inflightCall struct {
	wg   sync.WaitGroup
	item *Item
	err  error
}

// CachedClient wraps a Client with caching capabilities.
type CachedClient struct {
	client   *Client
	cacheDir string
	logger   Logger

	mu       sync.Mutex
	inflight map[int]*inflightCall
}

// CacheOption configures the CachedClient.
type CacheOption func(*CachedClient)

// WithLogger sets a custom Logger for the CachedClient.
func WithLogger(l Logger) CacheOption {
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
		logger:   &noopLogger{},
		inflight: make(map[int]*inflightCall),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c, nil
}

// GetItem retrieves an item by ID, using the cache if available.
func (c *CachedClient) GetItem(id int) (*Item, error) {
	// try read from cache (includes negative cache hits)
	item, err := c.readCache(id)
	if err == nil {
		c.logger.Info("cache hit for item %d", id)
		return item, nil
	}
	if errors.Is(err, ErrItemDeleted) || errors.Is(err, ErrItemDead) {
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
	call.item, call.err = c.client.GetItem(id)
	_ = c.writeCache(id, call.item, call.err)

	// signal waiting goroutines and cleanup
	c.mu.Lock()
	delete(c.inflight, id)
	c.mu.Unlock()
	call.wg.Done()

	return call.item, call.err
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
