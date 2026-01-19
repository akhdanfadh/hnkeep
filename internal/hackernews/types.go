package hackernews

import (
	"errors"
)

// Item represents a Hacker News item (story, comment, poll, etc.).
// Refer to https://github.com/HackerNews/API#items.
type Item struct {
	ID          int    `json:"id"`
	Deleted     bool   `json:"deleted,omitempty"`
	Type        string `json:"type,omitempty"`
	By          string `json:"by,omitempty"`
	Time        int64  `json:"time,omitempty"`
	Text        string `json:"text,omitempty"`
	Dead        bool   `json:"dead,omitempty"`
	Parent      int    `json:"parent,omitempty"`
	Poll        int    `json:"poll,omitempty"`
	Kids        []int  `json:"kids,omitempty"`
	URL         string `json:"url,omitempty"`
	Score       int    `json:"score,omitempty"`
	Title       string `json:"title,omitempty"`
	Parts       []int  `json:"parts,omitempty"`
	Descendants int    `json:"descendants,omitempty"`
}

var (
	// ErrItemNotFound is returned when the requested item does not exist.
	ErrItemNotFound = errors.New("item not found")
	// ErrItemDeleted is returned when the requested item is marked as deleted.
	ErrItemDeleted = errors.New("item is deleted")
	// ErrItemDead is returned when the requested item is marked as dead.
	ErrItemDead = errors.New("item is dead")
)
