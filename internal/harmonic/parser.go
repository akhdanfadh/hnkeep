package harmonic

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Bookmark represents a parsed bookmark from Harmonic-HN export.
type Bookmark struct {
	// Hacker News item ID. See https://github.com/HackerNews/API#items.
	ID int
	// Unix timestamp when bookmarked. Explicitly int64 to avoid overflow on 32-bit systems.
	// The original Harmonic export is in milliseconds. We convert to seconds here.
	Timestamp int64
}

// parseBookmark parses a single bookmark string of the format "{storyId}q{timestamp}"
func parseBookmark(s string) (Bookmark, error) {
	// separate and validate
	idStr, tsStr, found := strings.Cut(s, "q")
	if !found {
		return Bookmark{}, errors.New("missing 'q' separator")
	}
	if idStr == "" {
		return Bookmark{}, errors.New("missing item ID")
	}
	if tsStr == "" {
		return Bookmark{}, errors.New("missing timestamp")
	}

	// parse them
	id, err := strconv.Atoi(idStr)
	if err != nil {
		return Bookmark{}, fmt.Errorf("invalid item ID: %w", err)
	}
	if id <= 0 {
		return Bookmark{}, errors.New("item ID must be positive")
	}
	ts, err := strconv.ParseInt(tsStr, 10, 64) // not atoi bcs it's int64
	if err != nil {
		return Bookmark{}, fmt.Errorf("invalid timestamp: %w", err)
	}
	if ts <= 0 {
		return Bookmark{}, errors.New("timestamp must be positive")
	}

	return Bookmark{ID: id, Timestamp: ts / 1000}, nil
}

// Parse parses the Harmonic-HN export string.
// Format: {storyId}q{timestamp}-{storyId}q{timestamp}-...
func Parse(input string) ([]Bookmark, error) {
	input = strings.TrimSpace(input)
	input = strings.Trim(input, "-") // just to make sure
	if input == "" {
		return nil, errors.New("empty input")
	}

	parts := strings.Split(input, "-")
	bookmarks := make([]Bookmark, 0, len(parts))

	for i, part := range parts {
		part = strings.TrimSpace(part) // basic sanitation
		if part == "" {
			continue
		}

		bookmark, err := parseBookmark(part)
		if err != nil {
			return nil, fmt.Errorf("invalid bookmark at index %d: %w", i, err)
		}
		bookmarks = append(bookmarks, bookmark)
	}

	if len(bookmarks) == 0 {
		return nil, errors.New("no valid bookmarks found")
	}
	return bookmarks, nil
}
