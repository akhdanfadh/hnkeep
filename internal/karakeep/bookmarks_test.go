package karakeep

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ptr returns a pointer to the given string.
func ptr(s string) *string { return &s }

func TestClient_CreateBookmark(t *testing.T) {
	tests := map[string]struct {
		statusCode  int
		response    CreateBookmarkResponse
		rawResponse string // if set, write this instead of encoding response
		wantExists  bool
		wantErr     bool
		errContain  string
		errSentinel error
	}{
		"new bookmark created (201)": {
			statusCode: http.StatusCreated,
			response: CreateBookmarkResponse{
				ID:        "bm-123",
				CreatedAt: "2024-01-01T00:00:00Z",
				Title:     ptr("Test Title"),
			},
			wantExists: false,
		},
		"existing bookmark returned (200)": {
			statusCode: http.StatusOK,
			response: CreateBookmarkResponse{
				ID:        "bm-existing",
				CreatedAt: "2023-06-15T12:00:00Z",
				Title:     ptr("Existing Title"),
				Note:      ptr("existing note"),
			},
			wantExists: true,
		},
		"unauthorized (401)": {
			statusCode:  http.StatusUnauthorized,
			wantErr:     true,
			errSentinel: ErrUnauthorized,
		},
		"bad request (400)": {
			statusCode: http.StatusBadRequest,
			wantErr:    true,
			errContain: "HTTP 400",
		},
		"malformed JSON response": {
			statusCode:  http.StatusCreated,
			rawResponse: `{"id": "bm-123", "createdAt": `, // truncated JSON
			wantErr:     true,
			errContain:  "decoding response",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// verify request method and path
				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
				}
				if r.URL.Path != "/bookmarks" {
					t.Errorf("expected /bookmarks, got %s", r.URL.Path)
				}

				w.WriteHeader(tc.statusCode)
				if tc.statusCode == http.StatusCreated || tc.statusCode == http.StatusOK {
					if tc.rawResponse != "" {
						_, _ = w.Write([]byte(tc.rawResponse))
					} else {
						_ = json.NewEncoder(w).Encode(tc.response)
					}
				}
			}))
			defer server.Close()

			client := NewClient(server.URL, "test-key",
				WithHTTPClient(server.Client()),
				WithMaxRetries(1),
				WithRetryWait(0),
			)

			resp, exists, err := client.CreateBookmark(context.Background(),
				"https://example.com",
				"2024-01-01T00:00:00Z",
				ptr("Test Title"),
				nil,
			)

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.errSentinel != nil && !errors.Is(err, tc.errSentinel) {
					t.Errorf("expected error %v, got %v", tc.errSentinel, err)
				}
				if tc.errContain != "" && !strings.Contains(err.Error(), tc.errContain) {
					t.Errorf("expected error to contain %q, got %q", tc.errContain, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if exists != tc.wantExists {
				t.Errorf("exists = %v, want %v", exists, tc.wantExists)
			}
			if resp.ID != tc.response.ID {
				t.Errorf("response ID = %q, want %q", resp.ID, tc.response.ID)
			}
		})
	}
}

func TestClient_AttachTags(t *testing.T) {
	tests := map[string]struct {
		tags        []string
		statusCode  int
		wantErr     bool
		errSentinel error
		wantNoCall  bool // expect no HTTP call (empty tags optimization)
	}{
		"empty tags no-op": {
			tags:       []string{},
			wantNoCall: true,
		},
		"nil tags no-op": {
			tags:       nil,
			wantNoCall: true,
		},
		"success attaching tags": {
			tags:       []string{"hn", "imported"},
			statusCode: http.StatusOK,
		},
		"bookmark not found (404)": {
			tags:        []string{"tag1"},
			statusCode:  http.StatusNotFound,
			wantErr:     true,
			errSentinel: ErrBookmarkNotFound,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			called := false
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				called = true

				// verify request
				if r.Method != http.MethodPost {
					t.Errorf("expected POST, got %s", r.Method)
				}
				if !strings.HasSuffix(r.URL.Path, "/tags") {
					t.Errorf("expected path to end with /tags, got %s", r.URL.Path)
				}

				w.WriteHeader(tc.statusCode)
			}))
			defer server.Close()

			client := NewClient(server.URL, "test-key",
				WithHTTPClient(server.Client()),
				WithMaxRetries(1),
				WithRetryWait(0),
			)

			err := client.AttachTags(context.Background(), "bm-123", tc.tags)

			if tc.wantNoCall && called {
				t.Error("expected no HTTP call for empty tags")
			}
			if !tc.wantNoCall && !called && !tc.wantErr {
				t.Error("expected HTTP call but none was made")
			}

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.errSentinel != nil && !errors.Is(err, tc.errSentinel) {
					t.Errorf("expected error %v, got %v", tc.errSentinel, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestClient_UpdateBookmark(t *testing.T) {
	tests := map[string]struct {
		statusCode  int
		wantErr     bool
		errSentinel error
	}{
		"success": {
			statusCode: http.StatusOK,
		},
		"bookmark not found (404)": {
			statusCode:  http.StatusNotFound,
			wantErr:     true,
			errSentinel: ErrBookmarkNotFound,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// verify request
				if r.Method != http.MethodPatch {
					t.Errorf("expected PATCH, got %s", r.Method)
				}
				if !strings.HasPrefix(r.URL.Path, "/bookmarks/") {
					t.Errorf("expected path to start with /bookmarks/, got %s", r.URL.Path)
				}

				w.WriteHeader(tc.statusCode)
			}))
			defer server.Close()

			client := NewClient(server.URL, "test-key",
				WithHTTPClient(server.Client()),
				WithMaxRetries(1),
				WithRetryWait(0),
			)

			err := client.UpdateBookmark(context.Background(), "bm-123", ptr("2024-01-01T00:00:00Z"), ptr("updated note"))

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.errSentinel != nil && !errors.Is(err, tc.errSentinel) {
					t.Errorf("expected error %v, got %v", tc.errSentinel, err)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestClient_ListBookmarks(t *testing.T) {
	t.Run("fetches all bookmarks with pagination", func(t *testing.T) {
		pageCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodGet {
				t.Errorf("expected GET, got %s", r.Method)
			}
			if !strings.HasPrefix(r.URL.Path, "/bookmarks") {
				t.Errorf("expected /bookmarks path, got %s", r.URL.Path)
			}

			pageCount++
			w.WriteHeader(http.StatusOK)

			// page 1: return link and asset bookmarks with cursor
			if pageCount == 1 {
				cursor := "cursor-page-2"
				_ = json.NewEncoder(w).Encode(ListBookmarksResponse{
					Bookmarks: []ListBookmark{
						{
							ID:        "bm-link",
							CreatedAt: "2024-01-01T00:00:00Z",
							Note:      ptr("link note"),
							Content:   ListBookmarkContent{Type: "link", URL: ptr("https://example.com")},
						},
						{
							ID:        "bm-asset",
							CreatedAt: "2024-01-02T00:00:00Z",
							Content:   ListBookmarkContent{Type: "asset", SourceURL: ptr("https://example.com/doc.pdf")},
						},
					},
					NextCursor: &cursor,
				})
				return
			}

			// page 2: return text bookmark (should be skipped) and no cursor
			_ = json.NewEncoder(w).Encode(ListBookmarksResponse{
				Bookmarks: []ListBookmark{
					{
						ID:        "bm-text",
						CreatedAt: "2024-01-03T00:00:00Z",
						Content:   ListBookmarkContent{Type: "text"},
					},
					{
						ID:        "bm-link-2",
						CreatedAt: "2024-01-04T00:00:00Z",
						Content:   ListBookmarkContent{Type: "link", URL: ptr("https://another.com")},
					},
				},
				NextCursor: nil,
			})
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-key",
			WithHTTPClient(server.Client()),
			WithMaxRetries(1),
			WithRetryWait(0),
		)

		result, err := client.ListBookmarks(context.Background())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if pageCount != 2 {
			t.Errorf("expected 2 pages, got %d", pageCount)
		}

		// should have 3 entries: link, asset, link-2 (text skipped)
		if len(result) != 3 {
			t.Errorf("expected 3 bookmarks, got %d", len(result))
		}

		// verify link bookmark
		if bm, ok := result["https://example.com"]; !ok {
			t.Error("missing link bookmark")
		} else if bm.ID != "bm-link" {
			t.Errorf("link bookmark ID = %q, want %q", bm.ID, "bm-link")
		}

		// verify asset bookmark (PDF URL should be keyed)
		if bm, ok := result["https://example.com/doc.pdf"]; !ok {
			t.Error("missing asset bookmark")
		} else if bm.ID != "bm-asset" {
			t.Errorf("asset bookmark ID = %q, want %q", bm.ID, "bm-asset")
		}

		// verify text bookmark is NOT present
		if _, ok := result["bm-text"]; ok {
			t.Error("text bookmark should not be in result")
		}
	})

	t.Run("handles API error", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-key",
			WithHTTPClient(server.Client()),
			WithMaxRetries(1),
			WithRetryWait(0),
		)

		_, err := client.ListBookmarks(context.Background())
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if !strings.Contains(err.Error(), "listing bookmarks") {
			t.Errorf("expected error to mention listing bookmarks, got: %v", err)
		}
	})

	t.Run("respects context cancellation", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(ListBookmarksResponse{})
		}))
		defer server.Close()

		client := NewClient(server.URL, "test-key",
			WithHTTPClient(server.Client()),
		)

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := client.ListBookmarks(ctx)
		if err == nil {
			t.Fatal("expected error, got nil")
		}
	})
}
