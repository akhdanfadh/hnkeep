package hackernews

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"
)

const (
	defaultBaseURL    = "https://hacker-news.firebaseio.com/v0"
	defaultTimeout    = 10 * time.Second
	defaultMaxRetries = 3
	defaultRetryWait  = time.Second
)

// Client is a Hacker News API client.
type Client struct {
	httpClient *http.Client
	baseURL    string
	maxRetries int
	retryWait  time.Duration
}

// ClientOption configures the Client.
type ClientOption func(*Client)

// NewClient creates a new Hacker News API client with the given options.
func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		httpClient: &http.Client{Timeout: defaultTimeout},
		baseURL:    defaultBaseURL,
		maxRetries: defaultMaxRetries,
		retryWait:  defaultRetryWait,
	}

	// NOTE: Functional options pattern: allows callers to customize behavior
	// (e.g., in tests) while keeping NewClient() clean and simple for common case.
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// WithBaseURL sets a custom base URL for the Hacker News API (useful for testing).
func WithBaseURL(url string) ClientOption {
	return func(c *Client) {
		c.baseURL = url
	}
}

// WithRetries sets the maximum number of retries for requests.
func WithRetries(n int) ClientOption {
	return func(c *Client) {
		c.maxRetries = n
	}
}

// WithRetryWait sets the wait duration between retries.
func WithRetryWait(d time.Duration) ClientOption {
	return func(c *Client) {
		c.retryWait = d
	}
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = client
	}
}

// GetItem fetches an item by its ID with retry logic.
func (c *Client) GetItem(ctx context.Context, id int) (*Item, error) {
	url := fmt.Sprintf("%s/item/%d.json", c.baseURL, id)

	var lastErr error
	for attempt := 0; attempt < c.maxRetries; attempt++ {
		// check for cancellation before each attempt
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			// use a timer that respects context cancellation
			case <-time.After(c.retryWait):
			}
		}

		item, err := c.fetchItem(ctx, url)
		if err == nil {
			return item, nil // immediate return on success
		}

		if errors.Is(err, ErrItemNotFound) ||
			errors.Is(err, ErrItemDeleted) ||
			errors.Is(err, ErrItemDead) {
			return nil, err // immediate return on known errors
		}

		// context cancellation should return immediately
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}

		// exponential backoff capped at 30s for rate limiting
		if errors.Is(err, ErrRateLimited) {
			backoff := min(c.retryWait*time.Duration(1<<attempt), 30*time.Second)
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
			lastErr = err
			continue
		}

		lastErr = err
	}

	return nil, fmt.Errorf("failed after %d attempts: %w", c.maxRetries, lastErr)
}

// fetchItem performs the actual HTTP GET request to fetch the item.
func (c *Client) fetchItem(ctx context.Context, url string) (*Item, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	// NOTE: Close errors are not actionable here. The response body has already been
	// read and the actual HTTP operation succeeded or failed. Network errors during
	// close are transient and don't indicate application logic issues.
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, ErrRateLimited
	}

	var item Item
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}

	// NOTE: Turns out HN API always returns 200 OK (probably Firebase quirk, idk).
	// For non-existent items, it returns "null" in the body.
	if item.ID == 0 {
		return nil, ErrItemNotFound
	}

	if item.Deleted {
		return nil, ErrItemDeleted
	}

	if item.Dead {
		return nil, ErrItemDead
	}

	return &item, nil
}

// DiscussionURL returns the Hacker News discussion URL for the given item ID.
func DiscussionURL(id int) string {
	return "https://news.ycombinator.com/item?id=" + strconv.Itoa(id)
}
