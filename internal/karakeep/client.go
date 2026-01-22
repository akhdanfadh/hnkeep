package karakeep

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/akhdanfadh/hnkeep/internal/logger"
)

const (
	defaultTimeout    = 10 * time.Second
	defaultMaxRetries = 3
	defaultRetryWait  = time.Second
)

// Client is a Karakeep API client.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
	maxRetries int
	retryWait  time.Duration
	logger     logger.Logger
}

// ClientOption configures the Client.
type ClientOption func(*Client)

// NewClient creates a new Karakeep API client with the given base URL, API key, and options.
func NewClient(baseURL, apiKey string, opts ...ClientOption) *Client {
	c := &Client{
		baseURL:    strings.TrimRight(baseURL, "/"), // ensure no trailing slash
		apiKey:     apiKey,
		httpClient: &http.Client{Timeout: defaultTimeout},
		maxRetries: defaultMaxRetries,
		retryWait:  defaultRetryWait,
		logger:     logger.Noop(),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// WithHTTPClient sets a custom HTTP client.
func WithHTTPClient(client *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = client
	}
}

// WithMaxRetries sets the maximum number of retries for requests.
func WithMaxRetries(n int) ClientOption {
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

// WithLogger sets the logger for retry and rate limit visibility.
func WithLogger(l logger.Logger) ClientOption {
	return func(c *Client) {
		c.logger = l
	}
}

// waitWithContext waits for the specified duration or until context is cancelled.
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

// doRequestWithRetries performs the HTTP request with retries on failure.
//
// We implement exponential backoff for all retryable errors (rate limiting,
// network errors, server errors). There is no documentation for rate limiting
// in Karakeep API, but they do document it in practice for self-hosters.
// Refer to https://docs.karakeep.app/administration/security-considerations/.
func (c *Client) doRequestWithRetries(ctx context.Context, method, path string, body []byte, handleResp func(*http.Response) error) error {
	url := c.baseURL + path

	var lastErr error
	for attempt := 0; attempt < c.maxRetries; attempt++ {
		// check for cancellation before each attempt
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// do request and immediate return on non-retryable errors
		err := c.doRequest(ctx, method, url, body, handleResp)
		if err == nil {
			return nil // success
		}
		if errors.Is(err, ErrUnauthorized) || errors.Is(err, ErrBookmarkNotFound) {
			return err // known errors
		}
		var httpErr HTTPError
		if errors.As(err, &httpErr) && httpErr.IsClientError() {
			return err // client error
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err // context cancellation
		}

		// exponential backoff capped at 30s for all retryable errors
		backoff := min(c.retryWait*time.Duration(1<<attempt), 30*time.Second)
		if errors.Is(err, ErrRateLimited) {
			c.logger.Warn("rate limited, retrying in %s...", backoff)
		} else {
			c.logger.Warn("request failed (attempt %d/%d): %v, retrying in %s...", attempt+1, c.maxRetries, err, backoff)
		}

		if err := waitWithContext(ctx, backoff); err != nil {
			return err
		}
		lastErr = err
	}

	return fmt.Errorf("failed after %d attempts: %w", c.maxRetries, lastErr)
}

// doRequest performs a single HTTP request.
func (c *Client) doRequest(ctx context.Context, method, url string, body []byte, handleResp func(*http.Response) error) error {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}

	// NOTE: Karakeep API (built with Hono) always expects JSON request bodies
	// (validated via zValidator("json", ...)) and returns JSON responses via c.json().
	// Errors are returned as JSON via HTTPException with { message: string } format.
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }() // close error not actionable after body is read

	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	if resp.StatusCode == http.StatusTooManyRequests {
		return ErrRateLimited
	}

	return handleResp(resp)
}
