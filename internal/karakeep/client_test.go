package karakeep

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClient_doRequestWithRetries(t *testing.T) {
	tests := map[string]struct {
		responses    []int // sequence of status codes to return
		wantErr      bool
		errContain   string
		wantAttempts int
	}{
		"success on first attempt": {
			responses:    []int{http.StatusOK},
			wantAttempts: 1,
		},
		"unauthorized returns immediately": {
			responses:    []int{http.StatusUnauthorized},
			wantErr:      true,
			errContain:   "unauthorized",
			wantAttempts: 1,
		},
		"client error (4xx) returns immediately": {
			responses:    []int{http.StatusBadRequest},
			wantErr:      true,
			errContain:   "HTTP 400",
			wantAttempts: 1,
		},
		"server error (5xx) retries": {
			responses:    []int{http.StatusInternalServerError, http.StatusInternalServerError, http.StatusInternalServerError},
			wantErr:      true,
			errContain:   "failed after 3 attempts",
			wantAttempts: 3,
		},
		"server error then success": {
			responses:    []int{http.StatusInternalServerError, http.StatusOK},
			wantAttempts: 2,
		},
		"rate limited retries with backoff": {
			responses:    []int{http.StatusTooManyRequests, http.StatusTooManyRequests, http.StatusTooManyRequests},
			wantErr:      true,
			errContain:   "rate limited",
			wantAttempts: 3,
		},
		"rate limited then success": {
			responses:    []int{http.StatusTooManyRequests, http.StatusOK},
			wantAttempts: 2,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			attempts := 0
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				statusCode := tc.responses[attempts]
				attempts++
				w.WriteHeader(statusCode)
			}))
			defer server.Close()

			client := NewClient(server.URL, "test-api-key",
				WithHTTPClient(server.Client()),
				WithMaxRetries(3),
				WithRetryWait(0), // no wait for test speed
			)

			err := client.doRequestWithRetries(context.Background(), http.MethodGet, "/test", nil, func(resp *http.Response) error {
				if resp.StatusCode != http.StatusOK {
					return readHTTPError(resp)
				}
				return nil
			})

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.errContain != "" && !strings.Contains(err.Error(), tc.errContain) {
					t.Errorf("expected error to contain %q, got %q", tc.errContain, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}

			if attempts != tc.wantAttempts {
				t.Errorf("expected %d attempts, got %d", tc.wantAttempts, attempts)
			}
		})
	}
}

func TestClient_doRequestWithRetries_ContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond) // simulate slow response
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-api-key",
		WithHTTPClient(server.Client()),
		WithMaxRetries(3),
		WithRetryWait(time.Second),
	)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	err := client.doRequestWithRetries(ctx, http.MethodGet, "/test", nil, func(resp *http.Response) error {
		return nil
	})

	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}

func TestClient_doRequest_Headers(t *testing.T) {
	var capturedHeaders http.Header
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewClient(server.URL, "my-secret-key",
		WithHTTPClient(server.Client()),
	)

	err := client.doRequest(context.Background(), http.MethodPost, server.URL+"/test", []byte(`{"test":true}`), func(resp *http.Response) error {
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// verify authorization header
	authHeader := capturedHeaders.Get("Authorization")
	if authHeader != "Bearer my-secret-key" {
		t.Errorf("Authorization header = %q, want %q", authHeader, "Bearer my-secret-key")
	}

	// verify content-type for POST with body
	contentType := capturedHeaders.Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Content-Type header = %q, want %q", contentType, "application/json")
	}

	// verify accept header
	acceptHeader := capturedHeaders.Get("Accept")
	if acceptHeader != "application/json" {
		t.Errorf("Accept header = %q, want %q", acceptHeader, "application/json")
	}
}

func TestNewClient_TrimsTrailingSlash(t *testing.T) {
	client := NewClient("https://example.com/api/", "key")
	if client.baseURL != "https://example.com/api" {
		t.Errorf("baseURL = %q, want trailing slash trimmed", client.baseURL)
	}
}

func TestClient_CheckConnectivity(t *testing.T) {
	tests := map[string]struct {
		statusCode int
		wantErr    bool
		errContain string
	}{
		"success": {
			statusCode: http.StatusOK,
		},
		"unauthorized": {
			statusCode: http.StatusUnauthorized,
			wantErr:    true,
			errContain: "unauthorized",
		},
		"server error": {
			statusCode: http.StatusInternalServerError,
			wantErr:    true,
			errContain: "failed after",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// verify it's hitting the correct endpoint
				if r.URL.Path != "/users/me" {
					t.Errorf("unexpected path: %s, want /users/me", r.URL.Path)
				}
				if r.Method != http.MethodGet {
					t.Errorf("unexpected method: %s, want GET", r.Method)
				}
				w.WriteHeader(tc.statusCode)
			}))
			defer server.Close()

			client := NewClient(server.URL, "test-api-key",
				WithHTTPClient(server.Client()),
				WithMaxRetries(3),
				WithRetryWait(0),
			)

			err := client.CheckConnectivity(context.Background())

			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if tc.errContain != "" && !strings.Contains(err.Error(), tc.errContain) {
					t.Errorf("expected error to contain %q, got %q", tc.errContain, err.Error())
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
			}
		})
	}
}
