package hackernews

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewCachedClient(t *testing.T) {
	t.Run("valid path creates directory", func(t *testing.T) {
		cacheDir := t.TempDir()
		subDir := filepath.Join(cacheDir, "subdir", "cache")

		client := NewClient()
		cached, err := NewCachedClient(client, subDir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cached == nil {
			t.Fatal("expected client, got nil")
		}

		// verify directory was created
		info, err := os.Stat(subDir)
		if err != nil {
			t.Fatalf("cache directory not created: %v", err)
		}
		if !info.IsDir() {
			t.Fatal("expected directory, got file")
		}
	})

	t.Run("invalid path returns error", func(t *testing.T) {
		client := NewClient()
		_, err := NewCachedClient(client, "/dev/null/cache")
		if err == nil {
			t.Fatal("expected error for invalid path, got nil")
		}
	})
}

func TestCachedClient_GetItem_CacheMissAndHit(t *testing.T) {
	testItem := Item{
		ID:    12345,
		Title: "Test Article",
		URL:   "https://example.com",
		Time:  1688536396765,
	}

	var apiCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls.Add(1)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(testItem)
	}))
	defer server.Close()

	client := NewClient(
		WithHTTPClient(server.Client()),
		WithBaseURL(server.URL),
		WithRetries(1),
		WithRetryWait(0),
	)

	cacheDir := t.TempDir()
	cached, err := NewCachedClient(client, cacheDir)
	if err != nil {
		t.Fatalf("failed to create cached client: %v", err)
	}

	// first call: cache miss, should fetch from API
	item, err := cached.GetItem(context.Background(), 12345)
	if err != nil {
		t.Fatalf("first GetItem failed: %v", err)
	}
	if item.ID != testItem.ID || item.Title != testItem.Title {
		t.Errorf("unexpected item: got %+v, want %+v", item, testItem)
	}
	if apiCalls.Load() != 1 {
		t.Errorf("expected 1 API call on cache miss, got %d", apiCalls.Load())
	}

	// second call: cache hit, should NOT call API
	item, err = cached.GetItem(context.Background(), 12345)
	if err != nil {
		t.Fatalf("second GetItem failed: %v", err)
	}
	if item.ID != testItem.ID || item.Title != testItem.Title {
		t.Errorf("unexpected item from cache: got %+v, want %+v", item, testItem)
	}
	if apiCalls.Load() != 1 {
		t.Errorf("expected still 1 API call after cache hit, got %d", apiCalls.Load())
	}
}

func TestCachedClient_GetItem_NegativeCache_Deleted(t *testing.T) {
	deletedItem := Item{
		ID:      99999,
		Deleted: true,
	}

	var apiCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls.Add(1)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(deletedItem)
	}))
	defer server.Close()

	client := NewClient(
		WithHTTPClient(server.Client()),
		WithBaseURL(server.URL),
		WithRetries(1),
		WithRetryWait(0),
	)

	cacheDir := t.TempDir()
	cached, err := NewCachedClient(client, cacheDir)
	if err != nil {
		t.Fatalf("failed to create cached client: %v", err)
	}

	// first call: API returns deleted, should be cached
	_, err = cached.GetItem(context.Background(), 99999)
	if !errors.Is(err, ErrItemDeleted) {
		t.Fatalf("expected ErrItemDeleted, got %v", err)
	}
	if apiCalls.Load() != 1 {
		t.Errorf("expected 1 API call, got %d", apiCalls.Load())
	}

	// second call: should return cached error without API call
	_, err = cached.GetItem(context.Background(), 99999)
	if !errors.Is(err, ErrItemDeleted) {
		t.Fatalf("expected ErrItemDeleted from cache, got %v", err)
	}
	if apiCalls.Load() != 1 {
		t.Errorf("expected still 1 API call after negative cache hit, got %d", apiCalls.Load())
	}
}

func TestCachedClient_GetItem_NegativeCache_Dead(t *testing.T) {
	deadItem := Item{
		ID:   88888,
		Dead: true,
	}

	var apiCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls.Add(1)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(deadItem)
	}))
	defer server.Close()

	client := NewClient(
		WithHTTPClient(server.Client()),
		WithBaseURL(server.URL),
		WithRetries(1),
		WithRetryWait(0),
	)

	cacheDir := t.TempDir()
	cached, err := NewCachedClient(client, cacheDir)
	if err != nil {
		t.Fatalf("failed to create cached client: %v", err)
	}

	// first call: API returns dead, should be cached
	_, err = cached.GetItem(context.Background(), 88888)
	if !errors.Is(err, ErrItemDead) {
		t.Fatalf("expected ErrItemDead, got %v", err)
	}
	if apiCalls.Load() != 1 {
		t.Errorf("expected 1 API call, got %d", apiCalls.Load())
	}

	// second call: should return cached error without API call
	_, err = cached.GetItem(context.Background(), 88888)
	if !errors.Is(err, ErrItemDead) {
		t.Fatalf("expected ErrItemDead from cache, got %v", err)
	}
	if apiCalls.Load() != 1 {
		t.Errorf("expected still 1 API call after negative cache hit, got %d", apiCalls.Load())
	}
}

func TestCachedClient_GetItem_TransientErrorNotCached(t *testing.T) {
	var apiCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls.Add(1)
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(
		WithHTTPClient(server.Client()),
		WithBaseURL(server.URL),
		WithRetries(1),
		WithRetryWait(0),
	)

	cacheDir := t.TempDir()
	cached, err := NewCachedClient(client, cacheDir)
	if err != nil {
		t.Fatalf("failed to create cached client: %v", err)
	}

	// first call: transient error (500), should NOT be cached
	_, err = cached.GetItem(context.Background(), 77777)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if apiCalls.Load() != 1 {
		t.Errorf("expected 1 API call, got %d", apiCalls.Load())
	}

	// second call: should retry API (error was not cached)
	_, err = cached.GetItem(context.Background(), 77777)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if apiCalls.Load() != 2 {
		t.Errorf("expected 2 API calls (transient error not cached), got %d", apiCalls.Load())
	}
}

func TestCachedClient_GetItem_CorruptedCache(t *testing.T) {
	tests := map[string]struct {
		cacheContent string
	}{
		"invalid JSON": {
			cacheContent: "not valid json{",
		},
		"both item and error set": {
			cacheContent: `{"item":{"id":12345,"title":"Test"},"error":"deleted"}`,
		},
		"neither item nor error": {
			cacheContent: `{}`,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			testItem := Item{
				ID:    12345,
				Title: "Fresh Item",
				URL:   "https://example.com",
			}

			var apiCalls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				apiCalls.Add(1)
				w.WriteHeader(http.StatusOK)
				_ = json.NewEncoder(w).Encode(testItem)
			}))
			defer server.Close()

			client := NewClient(
				WithHTTPClient(server.Client()),
				WithBaseURL(server.URL),
				WithRetries(1),
				WithRetryWait(0),
			)

			cacheDir := t.TempDir()
			cached, err := NewCachedClient(client, cacheDir)
			if err != nil {
				t.Fatalf("failed to create cached client: %v", err)
			}

			// write corrupted cache file
			cachePath := filepath.Join(cacheDir, "12345.json")
			if err := os.WriteFile(cachePath, []byte(tc.cacheContent), 0o644); err != nil {
				t.Fatalf("failed to write corrupted cache: %v", err)
			}

			// GetItem should gracefully fall back to API
			item, err := cached.GetItem(context.Background(), 12345)
			if err != nil {
				t.Fatalf("GetItem failed: %v", err)
			}
			if item.ID != testItem.ID || item.Title != testItem.Title {
				t.Errorf("unexpected item: got %+v, want %+v", item, testItem)
			}
			if apiCalls.Load() != 1 {
				t.Errorf("expected 1 API call after corrupted cache, got %d", apiCalls.Load())
			}
		})
	}
}

func TestCachedClient_ClearCache(t *testing.T) {
	testItem := Item{
		ID:    55555,
		Title: "Cached Item",
		URL:   "https://example.com",
	}

	var apiCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls.Add(1)
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(testItem)
	}))
	defer server.Close()

	client := NewClient(
		WithHTTPClient(server.Client()),
		WithBaseURL(server.URL),
		WithRetries(1),
		WithRetryWait(0),
	)

	cacheDir := t.TempDir()
	cached, err := NewCachedClient(client, cacheDir)
	if err != nil {
		t.Fatalf("failed to create cached client: %v", err)
	}

	// populate cache
	_, err = cached.GetItem(context.Background(), 55555)
	if err != nil {
		t.Fatalf("initial GetItem failed: %v", err)
	}
	if apiCalls.Load() != 1 {
		t.Errorf("expected 1 API call, got %d", apiCalls.Load())
	}

	// verify cache hit
	_, err = cached.GetItem(context.Background(), 55555)
	if err != nil {
		t.Fatalf("cached GetItem failed: %v", err)
	}
	if apiCalls.Load() != 1 {
		t.Errorf("expected still 1 API call, got %d", apiCalls.Load())
	}

	// clear cache
	if err := cached.ClearCache(); err != nil {
		t.Fatalf("ClearCache failed: %v", err)
	}

	// verify fresh fetch after clear
	_, err = cached.GetItem(context.Background(), 55555)
	if err != nil {
		t.Fatalf("GetItem after clear failed: %v", err)
	}
	if apiCalls.Load() != 2 {
		t.Errorf("expected 2 API calls after cache clear, got %d", apiCalls.Load())
	}
}

func TestCachedClient_GetItem_ConcurrentSameID(t *testing.T) {
	testItem := Item{
		ID:    12345,
		Title: "Concurrent Test",
		URL:   "https://example.com",
	}

	var apiCalls atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiCalls.Add(1)
		time.Sleep(50 * time.Millisecond) // simulate delay to ensure goroutines overlap
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(testItem)
	}))
	defer server.Close()

	client := NewClient(
		WithHTTPClient(server.Client()),
		WithBaseURL(server.URL),
		WithRetries(1),
		WithRetryWait(0),
	)

	cacheDir := t.TempDir()
	cached, err := NewCachedClient(client, cacheDir)
	if err != nil {
		t.Fatalf("failed to create cached client: %v", err)
	}

	// launch multiple goroutines requesting the same item ID
	const numGoroutines = 10
	var wg sync.WaitGroup
	wg.Add(numGoroutines)

	results := make(chan *Item, numGoroutines)
	errs := make(chan error, numGoroutines)
	for range numGoroutines {
		go func() {
			defer wg.Done()
			item, err := cached.GetItem(context.Background(), 12345)
			if err != nil {
				errs <- err
				return
			}
			results <- item
		}()
	}

	wg.Wait()
	close(results)
	close(errs)

	// check for errors
	for err := range errs {
		t.Errorf("unexpected error: %v", err)
	}

	// verify all goroutines got the same result
	for item := range results {
		if item.ID != testItem.ID || item.Title != testItem.Title {
			t.Errorf("unexpected item: got %+v, want %+v", item, testItem)
		}
	}

	// singleflight should ensure only one API call was made
	if apiCalls.Load() != 1 {
		t.Errorf("expected 1 API call with concurrent requests, got %d", apiCalls.Load())
	}
}
