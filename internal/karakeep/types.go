package karakeep

import (
	"errors"
	"fmt"
	"io"
	"net/http"
)

// NOTE: The term sentinel for "sentinel errors" comes from the concept of a
// sentinel value in CS, which is a special value that marks a boundary or
// signals a particular condition. Like a guard (sentinel) standing watch,
// these errors "stand out" as recognizable markers for specific situations,
// rather than being generic error messages that require string parsing to
// undertand. In Go, they are typically defined as package-level variables
// of type error and compared using errors.Is().

// Sentinel errors for specific API conditions.
var (
	ErrUnauthorized     = errors.New("unauthorized: invalid or missing API key")
	ErrBookmarkNotFound = errors.New("bookmark not found")
	ErrRateLimited      = errors.New("rate limited: too many requests")
)

// HTTPError represents an HTTP error from the API with status code and response body.
//
// NOTE: We store the raw Body string rather than parsing a specific JSON structure because
// Karakeep's error responses vary. Storing raw body ensures we capture all error details for debugging.
// - Format 1: manual JSON response (search `c.json` in packages/api/routes/*.ts)
// - Format 2: Hono's HTTP exception (search `throw new HTTPException` in packages/api/middlewares/*.ts)
// - Format 3: Hono's Zod validation errors (search `zValidator“ in packages/api/routes/*.ts)
// For reference, Hono is the web framework used by Karakeep: https://hono.dev/.
type HTTPError struct {
	StatusCode int
	Body       string
}

// Error implements the error interface for HTTPError.
func (e HTTPError) Error() string {
	return fmt.Sprintf("karakeep API error (HTTP %d): %s", e.StatusCode, e.Body)
}

// IsClientError returns true for 4xx HTTP status codes.
func (e HTTPError) IsClientError() bool {
	return e.StatusCode >= 400 && e.StatusCode < 500
}

// readHTTPError reads the response body and returns an HTTPError.
// It adds useful debug info of this rare edge case; body is not structured anyway.
func readHTTPError(resp *http.Response) HTTPError {
	body, readErr := io.ReadAll(resp.Body)
	bodyStr := string(body)
	if readErr != nil {
		bodyStr += fmt.Sprintf(" (body read error: %v)", readErr)
	}
	return HTTPError{StatusCode: resp.StatusCode, Body: bodyStr}
}

// CreateBookmarkRequest represents the request body to create a link-type bookmark.
type CreateBookmarkRequest struct {
	Type      string  `json:"type"`            // set to "link"
	Source    string  `json:"source"`          // set to "api"
	URL       string  `json:"url"`             // required
	CreatedAt string  `json:"createdAt"`       // when it is saved on harmonic (ISO8601)
	Title     *string `json:"title,omitempty"` // HN title nullable
	Note      *string `json:"note,omitempty"`  // converted's note nullable
}

func NewCreateBookmarkRequest(url, createdAt string, title, note *string) *CreateBookmarkRequest {
	return &CreateBookmarkRequest{
		Type:      "link",
		Source:    "api",
		URL:       url,
		CreatedAt: createdAt,
		Title:     title,
		Note:      note,
	}
}

// CreateBookmarkResponse represents a successful response body when creating or retrieving a bookmark.
type CreateBookmarkResponse struct {
	ID        string  `json:"id"`
	CreatedAt string  `json:"createdAt"` // ISO8601
	Title     *string `json:"title"`     // nullable
	Note      *string `json:"note"`      // nullable
}

// AttachTagsRequest represents the request body to attach tags to a bookmark.
type AttachTagsRequest struct {
	Tags []TagRequest `json:"tags"`
}

// TagRequest represents a tag to attach to a bookmark.
type TagRequest struct {
	TagName string `json:"tagName"`
}

// UpdateBookmarkRequest represents the request body to update a bookmark's note and/or createdAt.
type UpdateBookmarkRequest struct {
	CreatedAt *string `json:"createdAt,omitempty"` // nullable, ISO8601
	Note      *string `json:"note,omitempty"`      // nullable
}
