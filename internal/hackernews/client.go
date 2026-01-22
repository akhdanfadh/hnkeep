package hackernews

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/akhdanfadh/hnkeep/internal/logger"
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
	logger     logger.Logger
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
		logger:     logger.Noop(),
	}

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

// WithLogger sets the logger for retry and rate limit visibility.
func WithLogger(l logger.Logger) ClientOption {
	return func(c *Client) {
		c.logger = l
	}
}

// waitWithContext waits for the specified duration or until context is cancelled.
// Uses NewTimer instead of time.After to avoid memory leak before Go 1.23 for explicitness.
func waitWithContext(ctx context.Context, d time.Duration) error {
	timer := time.NewTimer(d)
	select {
	case <-ctx.Done():
		timer.Stop()
		return ctx.Err()
	case <-timer.C:
		return nil
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

		item, err := c.fetchItem(ctx, url)
		if err == nil {
			return item, nil // immediate return on success
		}

		if errors.Is(err, ErrItemNotFound) ||
			errors.Is(err, ErrItemDeleted) ||
			errors.Is(err, ErrItemDead) {
			return nil, err // immediate return on known errors
		}

		if ctx.Err() != nil {
			return nil, ctx.Err() // user cancelled
		}

		// exponential backoff capped at 30s for all retryable errors
		backoff := min(c.retryWait*time.Duration(1<<attempt), 30*time.Second)
		if errors.Is(err, ErrRateLimited) {
			c.logger.Warn("rate limited, retrying in %s...", backoff)
		} else {
			c.logger.Warn("request failed (attempt %d/%d): %v, retrying in %s...", attempt+1, c.maxRetries, err, backoff)
		}

		if err := waitWithContext(ctx, backoff); err != nil {
			return nil, err
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
	defer func() { _ = resp.Body.Close() }() // close error not actionable after read

	if resp.StatusCode == http.StatusTooManyRequests {
		return nil, ErrRateLimited
	}

	var item Item
	if err := json.NewDecoder(resp.Body).Decode(&item); err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}

	if item.ID == 0 { // HN API returns 200 with "null" body for missing items
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
