package syncer

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/akhdanfadh/hnkeep/internal/converter"
	"github.com/akhdanfadh/hnkeep/internal/karakeep"
)

// ptr returns a pointer to the given string.
func ptr(s string) *string { return &s }

func TestMergeNotes(t *testing.T) {
	tests := map[string]struct {
		existing    *string
		incoming    *string
		wantMerged  *string
		wantUpdated bool
	}{
		"nil incoming returns existing unchanged": {
			existing:    ptr("existing note"),
			incoming:    nil,
			wantMerged:  ptr("existing note"),
			wantUpdated: false,
		},
		"empty incoming returns existing unchanged": {
			existing:    ptr("existing note"),
			incoming:    ptr(""),
			wantMerged:  ptr("existing note"),
			wantUpdated: false,
		},
		"existing contains incoming (idempotent)": {
			existing:    ptr("my note with content"),
			incoming:    ptr("content"),
			wantMerged:  ptr("my note with content"),
			wantUpdated: false,
		},
		"nil existing replaced by incoming": {
			existing:    nil,
			incoming:    ptr("new note"),
			wantMerged:  ptr("new note"),
			wantUpdated: true,
		},
		"empty existing replaced by incoming": {
			existing:    ptr(""),
			incoming:    ptr("new note"),
			wantMerged:  ptr("new note"),
			wantUpdated: true,
		},
		"non-empty existing merged with separator": {
			existing:    ptr("first note"),
			incoming:    ptr("second note"),
			wantMerged:  ptr("first note\n\n---\n\nsecond note"),
			wantUpdated: true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			merged, updated := mergeNotes(tc.existing, tc.incoming)

			if updated != tc.wantUpdated {
				t.Errorf("mergeNotes() updated = %v, want %v", updated, tc.wantUpdated)
			}

			if (merged == nil) != (tc.wantMerged == nil) {
				t.Errorf("mergeNotes() merged nil mismatch: got nil=%v, want nil=%v", merged == nil, tc.wantMerged == nil)
				return
			}

			if merged != nil && *merged != *tc.wantMerged {
				t.Errorf("mergeNotes() merged = %q, want %q", *merged, *tc.wantMerged)
			}
		})
	}
}

func TestTimestampConversion(t *testing.T) {
	t.Run("unixToISO8601", func(t *testing.T) {
		// 2024-01-01 00:00:00 UTC
		got := unixToISO8601(1704067200)
		// RFC3339 format includes timezone
		if !strings.HasPrefix(got, "2024-01-01") {
			t.Errorf("unixToISO8601(1704067200) = %q, expected date 2024-01-01", got)
		}
	})

	t.Run("iso8601ToUnix", func(t *testing.T) {
		got, err := iso8601ToUnix("2024-01-01T00:00:00Z")
		if err != nil {
			t.Fatalf("iso8601ToUnix() error: %v", err)
		}
		if got != 1704067200 {
			t.Errorf("iso8601ToUnix() = %d, want 1704067200", got)
		}
	})

	t.Run("iso8601ToUnix invalid format", func(t *testing.T) {
		_, err := iso8601ToUnix("not-a-date")
		if err == nil {
			t.Error("iso8601ToUnix() expected error for invalid format")
		}
	})

	t.Run("roundtrip", func(t *testing.T) {
		original := int64(1704067200)
		iso := unixToISO8601(original)
		roundtrip, err := iso8601ToUnix(iso)
		if err != nil {
			t.Fatalf("roundtrip error: %v", err)
		}
		if roundtrip != original {
			t.Errorf("roundtrip failed: got %d, want %d", roundtrip, original)
		}
	})
}

func TestSync(t *testing.T) {
	t.Run("processes all bookmarks with mixed results", func(t *testing.T) {
		var mu sync.Mutex
		responses := map[string]struct {
			createStatus int
			createResp   karakeep.CreateBookmarkResponse
		}{
			"https://new.com": {
				createStatus: http.StatusCreated,
				createResp:   karakeep.CreateBookmarkResponse{ID: "bm-1", CreatedAt: "2024-01-01T00:00:00Z"},
			},
			"https://existing.com": {
				createStatus: http.StatusOK,
				createResp:   karakeep.CreateBookmarkResponse{ID: "bm-2", CreatedAt: "2023-01-01T00:00:00Z", Note: ptr("existing note")},
			},
			"https://skip.com": {
				createStatus: http.StatusOK,
				createResp:   karakeep.CreateBookmarkResponse{ID: "bm-3", CreatedAt: "2020-01-01T00:00:00Z"},
			},
			"https://timestamp-update.com": {
				createStatus: http.StatusOK,
				createResp:   karakeep.CreateBookmarkResponse{ID: "bm-4", CreatedAt: "2025-01-01T00:00:00Z"}, // NEWER than incoming
			},
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			defer mu.Unlock()

			if r.Method == http.MethodPost && r.URL.Path == "/bookmarks" {
				var req karakeep.CreateBookmarkRequest
				_ = json.NewDecoder(r.Body).Decode(&req)

				if resp, ok := responses[req.URL]; ok {
					w.WriteHeader(resp.createStatus)
					_ = json.NewEncoder(w).Encode(resp.createResp)
					return
				}
				w.WriteHeader(http.StatusInternalServerError)
				return
			}

			if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/tags") {
				w.WriteHeader(http.StatusOK)
				return
			}

			if r.Method == http.MethodPatch {
				w.WriteHeader(http.StatusOK)
				return
			}

			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := karakeep.NewClient(server.URL, "test-key",
			karakeep.WithHTTPClient(server.Client()),
			karakeep.WithMaxRetries(1),
			karakeep.WithRetryWait(0),
		)

		syncer := New(client, WithConcurrency(2))

		bookmarks := []converter.Bookmark{
			{
				CreatedAt: 1704067200, // 2024-01-01
				Title:     ptr("New Bookmark"),
				Content:   converter.NewBookmarkContent("https://new.com"),
				Tags:      []string{"tag1"},
			},
			{
				CreatedAt: 1704067200, // 2024-01-01, newer than existing's 2023
				Title:     ptr("Existing with update"),
				Content:   converter.NewBookmarkContent("https://existing.com"),
				Note:      ptr("new note to merge"),
			},
			{
				CreatedAt: 1704067200, // 2024-01-01, newer than skip's 2020, but no note to merge
				Title:     ptr("Skip this one"),
				Content:   converter.NewBookmarkContent("https://skip.com"),
			},
			{
				CreatedAt: 1704067200, // 2024-01-01, OLDER than existing's 2025 -> timestamp update
				Title:     ptr("Timestamp update"),
				Content:   converter.NewBookmarkContent("https://timestamp-update.com"),
			},
		}

		status := syncer.Sync(context.Background(), bookmarks)

		// new.com -> created (201)
		// existing.com -> updated (note merged)
		// skip.com -> skipped (incoming 2024 is NEWER than existing 2020, no update; no note)
		// timestamp-update.com -> updated (incoming 2024 is OLDER than existing 2025)
		if status[SyncCreated] != 1 {
			t.Errorf("SyncCreated = %d, want 1", status[SyncCreated])
		}
		if status[SyncUpdated] != 2 {
			t.Errorf("SyncUpdated = %d, want 2", status[SyncUpdated])
		}
		if status[SyncSkipped] != 1 {
			t.Errorf("SyncSkipped = %d, want 1", status[SyncSkipped])
		}
	})

	t.Run("handles CreateBookmark failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client := karakeep.NewClient(server.URL, "test-key",
			karakeep.WithHTTPClient(server.Client()),
			karakeep.WithMaxRetries(1),
			karakeep.WithRetryWait(0),
		)

		syncer := New(client, WithConcurrency(1))

		bookmarks := []converter.Bookmark{
			{
				CreatedAt: 1704067200,
				Title:     ptr("Will fail"),
				Content:   converter.NewBookmarkContent("https://fail.com"),
			},
		}

		status := syncer.Sync(context.Background(), bookmarks)

		if status[SyncFailed] != 1 {
			t.Errorf("SyncFailed = %d, want 1", status[SyncFailed])
		}
	})

	t.Run("handles AttachTags failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && r.URL.Path == "/bookmarks" {
				w.WriteHeader(http.StatusCreated)
				_ = json.NewEncoder(w).Encode(karakeep.CreateBookmarkResponse{
					ID:        "bm-1",
					CreatedAt: "2024-01-01T00:00:00Z",
				})
				return
			}
			if strings.HasSuffix(r.URL.Path, "/tags") {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := karakeep.NewClient(server.URL, "test-key",
			karakeep.WithHTTPClient(server.Client()),
			karakeep.WithMaxRetries(1),
			karakeep.WithRetryWait(0),
		)

		syncer := New(client, WithConcurrency(1))

		bookmarks := []converter.Bookmark{
			{
				CreatedAt: 1704067200,
				Title:     ptr("Tag fail"),
				Content:   converter.NewBookmarkContent("https://tagfail.com"),
				Tags:      []string{"will-fail"},
			},
		}

		status := syncer.Sync(context.Background(), bookmarks)

		if status[SyncFailed] != 1 {
			t.Errorf("SyncFailed = %d, want 1", status[SyncFailed])
		}
	})

	t.Run("handles UpdateBookmark failure", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && r.URL.Path == "/bookmarks" {
				w.WriteHeader(http.StatusOK) // existing bookmark
				_ = json.NewEncoder(w).Encode(karakeep.CreateBookmarkResponse{
					ID:        "bm-existing",
					CreatedAt: "2025-01-01T00:00:00Z", // newer than incoming
				})
				return
			}
			if r.Method == http.MethodPatch {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := karakeep.NewClient(server.URL, "test-key",
			karakeep.WithHTTPClient(server.Client()),
			karakeep.WithMaxRetries(1),
			karakeep.WithRetryWait(0),
		)

		syncer := New(client, WithConcurrency(1))

		bookmarks := []converter.Bookmark{
			{
				CreatedAt: 1704067200, // 2024-01-01, older than existing's 2025
				Title:     ptr("Update fail"),
				Content:   converter.NewBookmarkContent("https://updatefail.com"),
			},
		}

		status := syncer.Sync(context.Background(), bookmarks)

		if status[SyncFailed] != 1 {
			t.Errorf("SyncFailed = %d, want 1", status[SyncFailed])
		}
	})

	t.Run("handles malformed CreatedAt from API", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost && r.URL.Path == "/bookmarks" {
				w.WriteHeader(http.StatusOK) // existing bookmark
				_ = json.NewEncoder(w).Encode(karakeep.CreateBookmarkResponse{
					ID:        "bm-bad-date",
					CreatedAt: "not-a-valid-timestamp",
				})
				return
			}
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := karakeep.NewClient(server.URL, "test-key",
			karakeep.WithHTTPClient(server.Client()),
			karakeep.WithMaxRetries(1),
			karakeep.WithRetryWait(0),
		)

		syncer := New(client, WithConcurrency(1))

		bookmarks := []converter.Bookmark{
			{
				CreatedAt: 1704067200,
				Title:     ptr("Bad date"),
				Content:   converter.NewBookmarkContent("https://baddate.com"),
			},
		}

		status := syncer.Sync(context.Background(), bookmarks)

		if status[SyncFailed] != 1 {
			t.Errorf("SyncFailed = %d, want 1", status[SyncFailed])
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		requestCount := 0
		var mu sync.Mutex
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			requestCount++
			mu.Unlock()
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(karakeep.CreateBookmarkResponse{ID: "bm-1", CreatedAt: "2024-01-01T00:00:00Z"})
		}))
		defer server.Close()

		client := karakeep.NewClient(server.URL, "test-key",
			karakeep.WithHTTPClient(server.Client()),
			karakeep.WithMaxRetries(1),
			karakeep.WithRetryWait(0),
		)

		syncer := New(client, WithConcurrency(1))

		// create many bookmarks
		var bookmarks []converter.Bookmark
		for range 100 {
			bookmarks = append(bookmarks, converter.Bookmark{
				CreatedAt: 1704067200,
				Title:     ptr("Test"),
				Content:   converter.NewBookmarkContent("https://example.com"),
			})
		}

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately

		syncer.Sync(ctx, bookmarks)

		mu.Lock()
		count := requestCount
		mu.Unlock()

		// with immediate cancellation and concurrency 1, very few requests should complete
		if count > 10 {
			t.Errorf("expected few requests with cancelled context, got %d", count)
		}
	})

	t.Run("skips CreateBookmark API call when URL in pre-fetched map", func(t *testing.T) {
		var mu sync.Mutex
		createCalls := 0
		tagCalls := 0
		updateCalls := 0

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			defer mu.Unlock()

			if r.Method == http.MethodPost && r.URL.Path == "/bookmarks" {
				createCalls++
				// this should only be called for urls NOT in pre-fetched map
				var req karakeep.CreateBookmarkRequest
				_ = json.NewDecoder(r.Body).Decode(&req)
				w.WriteHeader(http.StatusCreated)
				_ = json.NewEncoder(w).Encode(karakeep.CreateBookmarkResponse{
					ID:        "bm-new",
					CreatedAt: "2024-01-01T00:00:00Z",
				})
				return
			}

			if r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/tags") {
				tagCalls++
				w.WriteHeader(http.StatusOK)
				return
			}

			if r.Method == http.MethodPatch {
				updateCalls++
				w.WriteHeader(http.StatusOK)
				return
			}

			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := karakeep.NewClient(server.URL, "test-key",
			karakeep.WithHTTPClient(server.Client()),
			karakeep.WithMaxRetries(1),
			karakeep.WithRetryWait(0),
		)

		// pre-fetched map simulates urls already in karakeep
		existingBookmarks := map[string]karakeep.ExistingBookmark{
			"https://existing.com": {
				ID:        "bm-existing",
				CreatedAt: 1704067200, // 2024-01-01
				Note:      nil,
			},
			"https://existing-with-note.com": {
				ID:        "bm-with-note",
				CreatedAt: 1704067200,
				Note:      ptr("existing note"),
			},
		}

		syncer := New(client,
			WithConcurrency(1),
			WithExistingBookmarks(existingBookmarks),
		)

		bookmarks := []converter.Bookmark{
			{
				// url in pre-fetch -> should skip CreateBookmark, only call AttachTags
				CreatedAt: 1704067200,
				Title:     ptr("Existing"),
				Content:   converter.NewBookmarkContent("https://existing.com"),
				Tags:      []string{"tag1"},
			},
			{
				// url NOT in pre-fetch -> should call CreateBookmark
				CreatedAt: 1704067200,
				Title:     ptr("New"),
				Content:   converter.NewBookmarkContent("https://new.com"),
				Tags:      []string{"tag2"},
			},
			{
				// url in pre-fetch with note merge -> should call UpdateBookmark
				CreatedAt: 1704067200,
				Title:     ptr("With note merge"),
				Content:   converter.NewBookmarkContent("https://existing-with-note.com"),
				Note:      ptr("new note to merge"),
			},
		}

		status := syncer.Sync(context.Background(), bookmarks)

		mu.Lock()
		defer mu.Unlock()

		// only 1 CreateBookmark call (for new.com), not 3
		if createCalls != 1 {
			t.Errorf("CreateBookmark calls = %d, want 1 (pre-fetch should skip 2)", createCalls)
		}

		// 2 AttachTags calls (existing.com and new.com have tags)
		if tagCalls != 2 {
			t.Errorf("AttachTags calls = %d, want 2", tagCalls)
		}

		// 1 UpdateBookmark call (existing-with-note.com needs note merge)
		if updateCalls != 1 {
			t.Errorf("UpdateBookmark calls = %d, want 1", updateCalls)
		}

		// results: 1 created, 1 updated, 1 skipped
		if status[SyncCreated] != 1 {
			t.Errorf("SyncCreated = %d, want 1", status[SyncCreated])
		}
		if status[SyncUpdated] != 1 {
			t.Errorf("SyncUpdated = %d, want 1", status[SyncUpdated])
		}
		if status[SyncSkipped] != 1 {
			t.Errorf("SyncSkipped = %d, want 1", status[SyncSkipped])
		}
	})
}
