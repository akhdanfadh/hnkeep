package converter

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/akhdanfadh/hnkeep/internal/hackernews"
	"github.com/akhdanfadh/hnkeep/internal/harmonic"
	"github.com/akhdanfadh/hnkeep/internal/karakeep"
)

// mockFetcher is a mock implementation of ItemFetcher for testing.
type mockFetcher struct {
	items  map[int]*hackernews.Item
	errors map[int]error
}

func (m *mockFetcher) GetItem(id int) (*hackernews.Item, error) {
	if err, ok := m.errors[id]; ok {
		return nil, err
	}
	if item, ok := m.items[id]; ok {
		return item, nil
	}
	return nil, hackernews.ErrItemNotFound
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
			var buf bytes.Buffer
			mock := &mockFetcher{items: tc.items, errors: tc.errors}
			c := New(WithFetcher(mock), WithConcurrency(2), WithOutput(&buf))

			got := c.FetchItems(tc.bookmarks)

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
			output := buf.String()
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
				{ID: 1, Timestamp: 1000000}, // ms
			},
			items: map[int]*hackernews.Item{
				1: {ID: 1, Title: "Story with URL", URL: "https://example.com"},
			},
			want: karakeep.Export{
				Bookmarks: []karakeep.Bookmark{
					{
						CreatedAt: 1000, // s
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
				{ID: 123, Timestamp: 2000000},
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
				{ID: 1, Timestamp: 1000000},
				{ID: 999, Timestamp: 2000000}, // not in items
				{ID: 2, Timestamp: 3000000},
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
				{ID: 1, Timestamp: 1000000},
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
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			c := New()
			got := c.Convert(tc.bookmarks, tc.items, tc.opts)

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
