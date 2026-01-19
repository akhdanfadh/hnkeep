package hackernews

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
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

// CachedClient wraps a Client with caching capabilities.
type CachedClient struct {
	client   *Client
	cacheDir string
}

// NewCachedClient creates a client that caches responses in the given directory.
func NewCachedClient(client *Client, cacheDir string) (*CachedClient, error) {
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, err
	}
	return &CachedClient{
		client:   client,
		cacheDir: cacheDir,
	}, nil
}

// GetItem retrieves an item by ID, using the cache if available.
func (c *CachedClient) GetItem(id int) (*Item, error) {
	// TODO: (potential race condition) if multiple goroutines call this
	// function with the same ID simultaneously, they could all miss cache,
	// all fetch from the API, and all write to the same file concurrently

	// try read from cache (includes negative cache hits)
	item, err := c.readCache(id)
	if err == nil {
		return item, nil
	}
	if errors.Is(err, ErrItemDeleted) || errors.Is(err, ErrItemDead) {
		return nil, err // cached error state
	}

	// fetch from API and cache result (best-effort)
	item, err = c.client.GetItem(id)
	_ = c.writeCache(id, item, err)
	return item, err
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
