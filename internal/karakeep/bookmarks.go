package karakeep

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

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
