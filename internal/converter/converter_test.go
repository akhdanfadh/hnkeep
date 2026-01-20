package converter

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/akhdanfadh/hnkeep/internal/hackernews"
	"github.com/akhdanfadh/hnkeep/internal/harmonic"
	"github.com/akhdanfadh/hnkeep/internal/karakeep"
)

// ptr returns a pointer to the given string (helper for test data).
func ptr(s string) *string { return &s }

// mockFetcher is a mock implementation of ItemFetcher for testing.
type mockFetcher struct {
	items  map[int]*hackernews.Item
	errors map[int]error
}

func (m *mockFetcher) GetItem(_ context.Context, id int) (*hackernews.Item, error) {
	if err, ok := m.errors[id]; ok {
		return nil, err
	}
	if item, ok := m.items[id]; ok {
		return item, nil
	}
	return nil, hackernews.ErrItemNotFound
}

// mockLogger is a mock implementation of Logger for testing.
type mockLogger struct {
	mu       sync.Mutex
	messages []string
}

func (m *mockLogger) Info(format string, args ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, "[INFO] "+fmt.Sprintf(format, args...))
}

func (m *mockLogger) Warn(format string, args ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, "[WARN] "+fmt.Sprintf(format, args...))
}

func (m *mockLogger) Error(format string, args ...any) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.messages = append(m.messages, "[ERROR] "+fmt.Sprintf(format, args...))
}

func (m *mockLogger) Output() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return strings.Join(m.messages, "\n")
}

func TestFetchItems(t *testing.T) {
	tests := map[string]struct {
		bookmarks      []harmonic.Bookmark
		items          map[int]*hackernews.Item
		errors         map[int]error
		wantItems      map[int]*hackernews.Item
		wantWarnings   []string
		noWantWarnings []string
	}{
		"single bookmark success": {
			bookmarks: []harmonic.Bookmark{
				{ID: 1, Timestamp: 1000},
			},
			items: map[int]*hackernews.Item{
				1: {ID: 1, Title: "Test Story", URL: "https://example.com"},
			},
			wantItems: map[int]*hackernews.Item{
				1: {ID: 1, Title: "Test Story", URL: "https://example.com"},
			},
		},
		"multiple bookmarks success": {
			bookmarks: []harmonic.Bookmark{
				{ID: 1, Timestamp: 1000},
				{ID: 2, Timestamp: 2000},
				{ID: 3, Timestamp: 3000},
			},
			items: map[int]*hackernews.Item{
				1: {ID: 1, Title: "Story 1", URL: "https://example1.com"},
				2: {ID: 2, Title: "Story 2", URL: "https://example2.com"},
				3: {ID: 3, Title: "Story 3", URL: "https://example3.com"},
			},
			wantItems: map[int]*hackernews.Item{
				1: {ID: 1, Title: "Story 1", URL: "https://example1.com"},
				2: {ID: 2, Title: "Story 2", URL: "https://example2.com"},
				3: {ID: 3, Title: "Story 3", URL: "https://example3.com"},
			},
		},
		"item not found": {
			bookmarks: []harmonic.Bookmark{
				{ID: 1, Timestamp: 1000},
				{ID: 999, Timestamp: 2000},
			},
			items: map[int]*hackernews.Item{
				1: {ID: 1, Title: "Story 1", URL: "https://example1.com"},
			},
			wantItems: map[int]*hackernews.Item{
				1: {ID: 1, Title: "Story 1", URL: "https://example1.com"},
			},
			wantWarnings: []string{"item 999 not found"},
		},
		"fetch error": {
			bookmarks: []harmonic.Bookmark{
				{ID: 1, Timestamp: 1000},
				{ID: 2, Timestamp: 2000},
			},
			items: map[int]*hackernews.Item{
				1: {ID: 1, Title: "Story 1", URL: "https://example1.com"},
			},
			errors: map[int]error{
				2: errors.New("network error"),
			},
			wantItems: map[int]*hackernews.Item{
				1: {ID: 1, Title: "Story 1", URL: "https://example1.com"},
			},
			wantWarnings:   []string{"failed to fetch item 2", "network error"},
			noWantWarnings: []string{"not found"},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			logger := &mockLogger{}
			mock := &mockFetcher{items: tc.items, errors: tc.errors}
			c := New(WithFetcher(mock), WithConcurrency(2), WithLogger(logger))

			got, err := c.FetchItems(context.Background(), tc.bookmarks)
			if err != nil {
				t.Fatalf("FetchItems() unexpected error: %v", err)
			}

			// check items count
			if len(got) != len(tc.wantItems) {
				t.Fatalf("FetchItems() got %d items, want %d", len(got), len(tc.wantItems))
			}

			// check each item
			for id, wantItem := range tc.wantItems {
				gotItem, ok := got[id]
				if !ok {
					t.Errorf("FetchItems() missing item %d", id)
					continue
				}
				if gotItem.ID != wantItem.ID || gotItem.Title != wantItem.Title || gotItem.URL != wantItem.URL {
					t.Errorf("FetchItems()[%d] = %+v, want %+v", id, gotItem, wantItem)
				}
			}

			// check warnings
			output := logger.Output()
			for _, warning := range tc.wantWarnings {
				if !strings.Contains(output, warning) {
					t.Errorf("FetchItems() output missing warning %q, got %q", warning, output)
				}
			}
			for _, warning := range tc.noWantWarnings {
				if strings.Contains(output, warning) {
					t.Errorf("FetchItems() output should not contain %q, got %q", warning, output)
				}
			}
		})
	}
}

func TestConvert(t *testing.T) {
	title1 := "Story with URL"
	title2 := "Story without URL"
	title3 := "Another Story"

	tests := map[string]struct {
		bookmarks []harmonic.Bookmark
		items     map[int]*hackernews.Item
		opts      Options
		want      karakeep.Export
	}{
		"single bookmark with URL": {
			bookmarks: []harmonic.Bookmark{
				{ID: 1, Timestamp: 1000},
			},
			items: map[int]*hackernews.Item{
				1: {ID: 1, Title: "Story with URL", URL: "https://example.com"},
			},
			want: karakeep.Export{
				Bookmarks: []karakeep.Bookmark{
					{
						CreatedAt: 1000,
						Title:     &title1,
						Content: &karakeep.BookmarkContent{
							Link: &karakeep.LinkContent{
								Type: karakeep.BookmarkTypeLink,
								URL:  "https://example.com",
							},
						},
					},
				},
			},
		},
		"single bookmark without URL (discussion link)": {
			bookmarks: []harmonic.Bookmark{
				{ID: 123, Timestamp: 2000},
			},
			items: map[int]*hackernews.Item{
				123: {ID: 123, Title: "Story without URL", URL: ""},
			},
			want: karakeep.Export{
				Bookmarks: []karakeep.Bookmark{
					{
						CreatedAt: 2000,
						Title:     &title2,
						Content: &karakeep.BookmarkContent{
							Link: &karakeep.LinkContent{
								Type: karakeep.BookmarkTypeLink,
								URL:  "https://news.ycombinator.com/item?id=123",
							},
						},
					},
				},
			},
		},
		"missing item skipped": {
			bookmarks: []harmonic.Bookmark{
				{ID: 1, Timestamp: 1000},
				{ID: 999, Timestamp: 2000}, // not in items
				{ID: 2, Timestamp: 3000},
			},
			items: map[int]*hackernews.Item{
				1: {ID: 1, Title: "Story with URL", URL: "https://example.com"},
				2: {ID: 2, Title: "Another Story", URL: "https://another.com"},
			},
			want: karakeep.Export{
				Bookmarks: []karakeep.Bookmark{
					{
						CreatedAt: 1000,
						Title:     &title1,
						Content: &karakeep.BookmarkContent{
							Link: &karakeep.LinkContent{
								Type: karakeep.BookmarkTypeLink,
								URL:  "https://example.com",
							},
						},
					},
					{
						CreatedAt: 3000,
						Title:     &title3,
						Content: &karakeep.BookmarkContent{
							Link: &karakeep.LinkContent{
								Type: karakeep.BookmarkTypeLink,
								URL:  "https://another.com",
							},
						},
					},
				},
			},
		},
		"bookmarks with tags": {
			bookmarks: []harmonic.Bookmark{
				{ID: 1, Timestamp: 1000},
			},
			items: map[int]*hackernews.Item{
				1: {ID: 1, Title: "Story with URL", URL: "https://example.com"},
			},
			opts: Options{Tags: []string{"hn", "imported"}},
			want: karakeep.Export{
				Bookmarks: []karakeep.Bookmark{
					{
						CreatedAt: 1000,
						Title:     &title1,
						Tags:      []string{"hn", "imported"},
						Content: &karakeep.BookmarkContent{
							Link: &karakeep.LinkContent{
								Type: karakeep.BookmarkTypeLink,
								URL:  "https://example.com",
							},
						},
					},
				},
			},
		},
		"note template empty string": {
			bookmarks: []harmonic.Bookmark{
				{ID: 1, Timestamp: 1000},
			},
			items: map[int]*hackernews.Item{
				1: {ID: 1, Title: "Story", URL: "https://example.com"},
			},
			opts: Options{NoteTemplate: ""},
			want: karakeep.Export{
				Bookmarks: []karakeep.Bookmark{
					{
						CreatedAt: 1000,
						Title:     ptr("Story"),
						Note:      nil, // no note when template is empty
						Content: &karakeep.BookmarkContent{
							Link: &karakeep.LinkContent{
								Type: karakeep.BookmarkTypeLink,
								URL:  "https://example.com",
							},
						},
					},
				},
			},
		},
		"note template smart_url with external URL": {
			bookmarks: []harmonic.Bookmark{
				{ID: 42, Timestamp: 1000},
			},
			items: map[int]*hackernews.Item{
				42: {ID: 42, Title: "Story", URL: "https://example.com"},
			},
			opts: Options{NoteTemplate: "{{smart_url}}"},
			want: karakeep.Export{
				Bookmarks: []karakeep.Bookmark{
					{
						CreatedAt: 1000,
						Title:     ptr("Story"),
						Note:      ptr("https://news.ycombinator.com/item?id=42"),
						Content: &karakeep.BookmarkContent{
							Link: &karakeep.LinkContent{
								Type: karakeep.BookmarkTypeLink,
								URL:  "https://example.com",
							},
						},
					},
				},
			},
		},
		"note template smart_url without external URL": {
			bookmarks: []harmonic.Bookmark{
				{ID: 99, Timestamp: 1000},
			},
			items: map[int]*hackernews.Item{
				99: {ID: 99, Title: "Ask HN: Something", URL: ""}, // no external URL
			},
			opts: Options{NoteTemplate: "{{smart_url}}"},
			want: karakeep.Export{
				Bookmarks: []karakeep.Bookmark{
					{
						CreatedAt: 1000,
						Title:     ptr("Ask HN: Something"),
						Note:      nil, // smart_url is empty, so note is not set
						Content: &karakeep.BookmarkContent{
							Link: &karakeep.LinkContent{
								Type: karakeep.BookmarkTypeLink,
								URL:  "https://news.ycombinator.com/item?id=99",
							},
						},
					},
				},
			},
		},
		"note template hn_url without external URL": {
			bookmarks: []harmonic.Bookmark{
				{ID: 88, Timestamp: 1000},
			},
			items: map[int]*hackernews.Item{
				88: {ID: 88, Title: "Ask HN: Question", URL: ""}, // no external URL
			},
			opts: Options{NoteTemplate: "{{hn_url}}"},
			want: karakeep.Export{
				Bookmarks: []karakeep.Bookmark{
					{
						CreatedAt: 1000,
						Title:     ptr("Ask HN: Question"),
						Note:      ptr("https://news.ycombinator.com/item?id=88"), // hn_url always works
						Content: &karakeep.BookmarkContent{
							Link: &karakeep.LinkContent{
								Type: karakeep.BookmarkTypeLink,
								URL:  "https://news.ycombinator.com/item?id=88",
							},
						},
					},
				},
			},
		},
		"note template with multiple variables": {
			bookmarks: []harmonic.Bookmark{
				{ID: 123, Timestamp: 1000},
			},
			items: map[int]*hackernews.Item{
				123: {
					ID:    123,
					Title: "Test Title",
					URL:   "https://example.com",
					By:    "testuser",
					Time:  1609459200, // 2021-01-01 00:00:00 UTC
				},
			},
			opts: Options{NoteTemplate: "{{title}} by {{author}} ({{date}}) - ID:{{id}} {{item_url}}"},
			want: karakeep.Export{
				Bookmarks: []karakeep.Bookmark{
					{
						CreatedAt: 1000,
						Title:     ptr("Test Title"),
						Note:      ptr("Test Title by testuser (2021-01-01) - ID:123 https://example.com"),
						Content: &karakeep.BookmarkContent{
							Link: &karakeep.LinkContent{
								Type: karakeep.BookmarkTypeLink,
								URL:  "https://example.com",
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			c := New()
			got, _ := c.Convert(tc.bookmarks, tc.items, tc.opts)

			// check bookmarks count
			if len(got.Bookmarks) != len(tc.want.Bookmarks) {
				t.Fatalf("Convert() got %d bookmarks, want %d", len(got.Bookmarks), len(tc.want.Bookmarks))
			}

			// check each bookmark
			for i, wantBm := range tc.want.Bookmarks {
				gotBm := got.Bookmarks[i]

				if gotBm.CreatedAt != wantBm.CreatedAt {
					t.Errorf("Convert()[%d].CreatedAt = %d, want %d", i, gotBm.CreatedAt, wantBm.CreatedAt)
				}

				if (gotBm.Title == nil) != (wantBm.Title == nil) {
					t.Errorf("Convert()[%d].Title nil mismatch", i)
				} else if gotBm.Title != nil && *gotBm.Title != *wantBm.Title {
					t.Errorf("Convert()[%d].Title = %q, want %q", i, *gotBm.Title, *wantBm.Title)
				}

				// check tags
				if len(gotBm.Tags) != len(wantBm.Tags) {
					t.Errorf("Convert()[%d].Tags length = %d, want %d", i, len(gotBm.Tags), len(wantBm.Tags))
				} else {
					for j, wantTag := range wantBm.Tags {
						if gotBm.Tags[j] != wantTag {
							t.Errorf("Convert()[%d].Tags[%d] = %q, want %q", i, j, gotBm.Tags[j], wantTag)
						}
					}
				}

				// check note
				if (gotBm.Note == nil) != (wantBm.Note == nil) {
					t.Errorf("Convert()[%d].Note nil mismatch: got nil=%v, want nil=%v", i, gotBm.Note == nil, wantBm.Note == nil)
				} else if gotBm.Note != nil && *gotBm.Note != *wantBm.Note {
					t.Errorf("Convert()[%d].Note = %q, want %q", i, *gotBm.Note, *wantBm.Note)
				}

				if (gotBm.Content == nil) != (wantBm.Content == nil) {
					t.Errorf("Convert()[%d].Content nil mismatch", i)
				} else if gotBm.Content != nil && gotBm.Content.Link != nil {
					if wantBm.Content.Link == nil {
						t.Errorf("Convert()[%d].Content.Link should be nil", i)
					} else {
						if gotBm.Content.Link.Type != wantBm.Content.Link.Type {
							t.Errorf("Convert()[%d].Content.Link.Type = %q, want %q", i, gotBm.Content.Link.Type, wantBm.Content.Link.Type)
						}
						if gotBm.Content.Link.URL != wantBm.Content.Link.URL {
							t.Errorf("Convert()[%d].Content.Link.URL = %q, want %q", i, gotBm.Content.Link.URL, wantBm.Content.Link.URL)
						}
					}
				}
			}
		})
	}
}

func TestConvert_Dedupe(t *testing.T) {
	t.Run("merges notes with separator", func(t *testing.T) {
		c := New()
		bookmarks := []harmonic.Bookmark{
			{ID: 1, Timestamp: 1000},
			{ID: 2, Timestamp: 2000},
		}
		items := map[int]*hackernews.Item{
			1: {ID: 1, Title: "First Story", URL: "https://example.com"},
			2: {ID: 2, Title: "Second Story", URL: "https://example.com"},
		}
		opts := Options{Dedupe: true, NoteTemplate: "{{hn_url}}"}

		got, deduped := c.Convert(bookmarks, items, opts)

		if len(got.Bookmarks) != 1 {
			t.Errorf("Convert() got %d bookmarks, want 1", len(got.Bookmarks))
		}
		if deduped != 1 {
			t.Errorf("Convert() deduped = %d, want 1", deduped)
		}
		if got.Bookmarks[0].Note == nil || !strings.Contains(*got.Bookmarks[0].Note, "---") {
			t.Errorf("Convert() note should contain separator, got %v", got.Bookmarks[0].Note)
		}
	})

	t.Run("duplicate note replaces empty first note", func(t *testing.T) {
		c := New()
		bookmarks := []harmonic.Bookmark{
			{ID: 1, Timestamp: 1000},
			{ID: 2, Timestamp: 2000},
		}
		// Item 1 has no external URL, resolves to HN discussion URL
		// Item 2 explicitly links to item 1's HN discussion URL
		items := map[int]*hackernews.Item{
			1: {ID: 1, Title: "Discussion Post", URL: ""},
			2: {ID: 2, Title: "Link to Discussion", URL: "https://news.ycombinator.com/item?id=1"},
		}
		// smart_url is empty when item has no external URL
		opts := Options{Dedupe: true, NoteTemplate: "{{smart_url}}"}

		got, deduped := c.Convert(bookmarks, items, opts)

		if len(got.Bookmarks) != 1 {
			t.Errorf("Convert() got %d bookmarks, want 1", len(got.Bookmarks))
		}
		if deduped != 1 {
			t.Errorf("Convert() deduped = %d, want 1", deduped)
		}
		// First item's note was empty, so duplicate's note should replace it (no separator)
		if got.Bookmarks[0].Note == nil {
			t.Error("Convert() note should not be nil")
		} else if strings.Contains(*got.Bookmarks[0].Note, "---") {
			t.Errorf("Convert() note should not contain separator when first was empty, got %q", *got.Bookmarks[0].Note)
		}
	})
}
