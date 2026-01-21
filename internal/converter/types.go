package converter

import "encoding/json"

// Schema represents the Karakeep export/import file schema.
// Refer to https://github.com/karakeep-app/karakeep/blob/main/packages/shared/import-export/exporters.ts
type Schema struct {
	Bookmarks []Bookmark `json:"bookmarks"`
}

// Bookmark represents a single bookmark in the Karakeep export/import file.
type Bookmark struct {
	CreatedAt int64            `json:"createdAt"` // Unix timestamp (in seconds)
	Title     *string          `json:"title"`     // Nullable
	Tags      BookmarkTags     `json:"tags"`      // Empty array if no tags
	Content   *BookmarkContent `json:"content"`   // Always link type, nullable
	Note      *string          `json:"note"`      // Nullable
}

// BookmarkTags is a custom type to handle marshaling empty arrays instead of null.
type BookmarkTags []string

func (s BookmarkTags) MarshalJSON() ([]byte, error) {
	if s == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]string(s))
}

// BookmarkContent represents the content of a link-type bookmark in Karakeep export/import file.
// The original schema supports discriminated union between "link" or "text" types,
// but we only use "link" type here for our use case.
type BookmarkContent struct {
	Type string `json:"type"`
	URL  string `json:"url"`
}

// NewBookmarkContent creates a new BookmarkContent with the given URL and the type field pre-set.
func NewBookmarkContent(url string) *BookmarkContent {
	return &BookmarkContent{Type: "link", URL: url}
}
