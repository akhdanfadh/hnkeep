package karakeep

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"
)

const listBookmarksPageSize = 100

// CreateBookmark creates a new link-type bookmark given the URL.
//
// If the URL is new, it creates the bookmark and returns it with exists=false.
// If the URL already exists, it returns the existing bookmark unedited with exists=true.
// Refer to https://docs.karakeep.app/api/create-a-new-bookmark and the codebase.
func (c *Client) CreateBookmark(ctx context.Context, url, createdAt string, title, note *string) (*CreateBookmarkResponse, bool, error) {
	reqBody := NewCreateBookmarkRequest(url, createdAt, title, note)
	data, err := json.Marshal(reqBody)
	if err != nil {
		return nil, false, fmt.Errorf("marshaling request: %w", err)
	}

	var karakeepBM CreateBookmarkResponse
	var alreadyExists bool

	err = c.doRequestWithRetries(ctx, http.MethodPost, "/bookmarks", data, func(resp *http.Response) error {
		alreadyExists = resp.StatusCode == http.StatusOK

		if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
			return readHTTPError(resp)
		}

		if err := json.NewDecoder(resp.Body).Decode(&karakeepBM); err != nil {
			return fmt.Errorf("decoding response: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, false, err
	}

	return &karakeepBM, alreadyExists, nil
}

// AttachTags attaches tags to an existing bookmark by its ID.
//
// The endpoint is idempotent, meaning existing tags are not duplicated, and new tags are added.
// Refer to https://docs.karakeep.app/api/attach-tags-to-a-bookmark and the codebase.
func (c *Client) AttachTags(ctx context.Context, id string, tags []string) error {
	if len(tags) == 0 {
		return nil // nothing to do
	}

	tagReqs := make([]TagRequest, len(tags))
	for i, tag := range tags {
		tagReqs[i] = TagRequest{TagName: tag}
	}

	reqBody := AttachTagsRequest{Tags: tagReqs}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	return c.doRequestWithRetries(ctx, http.MethodPost, "/bookmarks/"+id+"/tags", data, func(resp *http.Response) error {
		if resp.StatusCode == http.StatusNotFound {
			return ErrBookmarkNotFound
		}

		if resp.StatusCode != http.StatusOK {
			return readHTTPError(resp)
		}

		return nil
	})
}

// UpdateBookmark updates the note and/or createdAt values of an existing bookmark.
// Refer to https://docs.karakeep.app/api/update-a-bookmark and the codebase.
func (c *Client) UpdateBookmark(ctx context.Context, id string, createdAt, note *string) error {
	reqBody := UpdateBookmarkRequest{CreatedAt: createdAt, Note: note}
	data, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("marshaling request: %w", err)
	}

	return c.doRequestWithRetries(ctx, http.MethodPatch, "/bookmarks/"+id, data, func(resp *http.Response) error {
		if resp.StatusCode == http.StatusNotFound {
			return ErrBookmarkNotFound
		}

		if resp.StatusCode != http.StatusOK {
			return readHTTPError(resp)
		}

		return nil
	})
}

// ListBookmarks fetches all bookmarks and returns a map of URL to ExistingBookmark for deduplication.
// It handles pagination internally and extracts URLs from both link and asset content types.
// Refer to https://docs.karakeep.app/api/get-all-bookmarks and the codebase.
func (c *Client) ListBookmarks(ctx context.Context) (map[string]ExistingBookmark, error) {
	result := make(map[string]ExistingBookmark)
	var cursor string
	page := 1

	for {
		// check for cancellation
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		path := fmt.Sprintf("/bookmarks?limit=%d", listBookmarksPageSize)
		if cursor != "" {
			path += "&cursor=" + url.QueryEscape(cursor) // if not escaped, may break for special chars
		}

		var listResp ListBookmarksResponse
		err := c.doRequestWithRetries(ctx, http.MethodGet, path, nil, func(resp *http.Response) error {
			if resp.StatusCode != http.StatusOK {
				return readHTTPError(resp)
			}
			return json.NewDecoder(resp.Body).Decode(&listResp)
		})
		if err != nil {
			return nil, fmt.Errorf("listing bookmarks (page %d): %w", page, err)
		}

		for _, bm := range listResp.Bookmarks {
			bmURL := bm.Content.GetURL()
			if bmURL == "" {
				continue // skip text bookmarks
			}
			createdAt, err := iso8601ToUnix(bm.CreatedAt)
			if err != nil {
				continue // skip malformed entries
			}
			result[bmURL] = ExistingBookmark{
				ID:        bm.ID,
				CreatedAt: createdAt,
				Note:      bm.Note,
			}
		}

		if listResp.NextCursor == nil || *listResp.NextCursor == "" {
			break // no more pages
		}
		cursor = *listResp.NextCursor
		page++
	}

	return result, nil
}

// iso8601ToUnix converts an ISO8601 date string to a Unix timestamp (in seconds).
func iso8601ToUnix(iso string) (int64, error) {
	t, err := time.Parse(time.RFC3339, iso)
	if err != nil {
		return 0, fmt.Errorf("parsing ISO8601 date %q: %w", iso, err)
	}
	return t.Unix(), nil
}
