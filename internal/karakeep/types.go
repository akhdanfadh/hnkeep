package karakeep

import "encoding/json"

// Export represents the top-level export/import schema
type Export struct {
	Bookmarks []Bookmark `json:"bookmarks"`
}

// Bookmark represents a single bookmark in export/import
// Refer to https://github.com/karakeep-app/karakeep/blob/main/packages/shared/import-export/exporters.ts
type Bookmark struct {
	CreatedAt int64            `json:"createdAt"`          // Unix timestamp (in seconds)
	Title     *string          `json:"title"`              // Nullable
	Tags      Tags             `json:"tags"`               // Empty array if no tags
	Content   *BookmarkContent `json:"content"`            // Link object, text object, or null
	Note      *string          `json:"note"`               // Nullable
	Archived  bool             `json:"archived,omitempty"` // Defaults to false
}

// Tags is a custom type to handle marshaling empty arrays instead of null
type Tags []string

func (s Tags) MarshalJSON() ([]byte, error) {
	if s == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]string(s))
}

type BookmarkType string

const (
	BookmarkTypeLink BookmarkType = "link"
	BookmarkTypeText BookmarkType = "text"
	// no asset type as of v0.30.0
)

// LinkContent represents a bookmark with a URL
type LinkContent struct {
	Type BookmarkType `json:"type"`
	URL  string       `json:"url"`
}

// TextContent represents a bookmark with text content
type TextContent struct {
	Type BookmarkType `json:"type"`
	Text string       `json:"text"`
}

// BookmarkContent is a discriminated union for bookmark content.
// Fields use json:"-" so only our custom MarshalJSON/UnmarshalJSON runs.
type BookmarkContent struct {
	Link *LinkContent `json:"-"`
	Text *TextContent `json:"-"`
}

func (c *BookmarkContent) MarshalJSON() ([]byte, error) {
	if c == nil {
		return []byte("null"), nil
	}
	if c.Link != nil {
		return json.Marshal(c.Link)
	}
	if c.Text != nil {
		return json.Marshal(c.Text)
	}
	return []byte("null"), nil
}

func (c *BookmarkContent) UnmarshalJSON(data []byte) error {
	if string(data) == "null" {
		return nil
	}

	// determine the type first
	var typeOnly struct {
		Type BookmarkType `json:"type"`
	}
	if err := json.Unmarshal(data, &typeOnly); err != nil {
		return err
	}

	// for unknown type, karakeep parser is lenient (set to undefined),
	// our silent fallthrough mimics that behavior
	// refer to https://github.com/karakeep-app/karakeep/blob/main/packages/shared/import-export/parsers.ts
	switch typeOnly.Type {
	case BookmarkTypeLink:
		var link LinkContent
		if err := json.Unmarshal(data, &link); err != nil {
			return err
		}
		c.Link = &link
	case BookmarkTypeText:
		var text TextContent
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		c.Text = &text
	}
	return nil
}
