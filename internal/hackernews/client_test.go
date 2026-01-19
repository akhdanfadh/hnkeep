package hackernews

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestClient_GetItem(t *testing.T) {
	tests := map[string]struct {
		response   any
		statusCode int
		want       *Item
		wantErr    bool
		errContain string
	}{
		"successful response with URL": {
			response: Item{
				ID:    3742902,
				Title: "Test Article",
				URL:   "https://example.com/article",
				Time:  1688536396765,
			},
			statusCode: http.StatusOK,
			want: &Item{
				ID:    3742902,
				Title: "Test Article",
				URL:   "https://example.com/article",
				Time:  1688536396765,
			},
		},
		"text post without URL": {
			response: Item{
				ID:    3742902,
				Title: "Ask HN: Something?",
				URL:   "",
				Time:  1688536396765,
			},
			statusCode: http.StatusOK,
			want: &Item{
				ID:    3742902,
				Title: "Ask HN: Something?",
				URL:   "",
				Time:  1688536396765,
			},
		},
		"deleted item": {
			response: &Item{
				ID:      3742902,
				Deleted: true,
			},
			statusCode: http.StatusOK,
			wantErr:    true,
			errContain: "deleted",
		},
		"dead item": {
			response: &Item{
				ID:   3742902,
				Dead: true,
			},
			statusCode: http.StatusOK,
			wantErr:    true,
			errContain: "dead",
		},
		"null response (non-existent item)": {
			response:   nil,
			statusCode: http.StatusOK,
			wantErr:    true,
			errContain: "not found",
		},
		"invalid JSON response": {
			response:   "invalid json",
			statusCode: http.StatusOK,
			wantErr:    true,
			errContain: "decode failed",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			// mock server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				if tc.response != nil {
					_ = json.NewEncoder(w).Encode(tc.response)
				} else {
					_, _ = w.Write([]byte("null"))
				}
			}))
			defer server.Close()

			// custom client
			client := NewClient(
				WithHTTPClient(server.Client()),
				WithBaseURL(server.URL),
				WithRetries(1),
				WithRetryWait(0),
			)

			// check errors
			item, err := client.GetItem(3742902)
			if tc.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				} else if tc.errContain != "" && !strings.Contains(err.Error(), tc.errContain) {
					t.Fatalf("expected error to contain %q, got %q", tc.errContain, err.Error())
				}
				return
			}
			if err != nil {
				t.Fatal("unexpected error:", err)
			}

			// check result
			if item.ID != tc.want.ID {
				t.Errorf("expected ID %d, got %d", tc.want.ID, item.ID)
			}
			if item.Title != tc.want.Title {
				t.Errorf("expected Title %q, got %q", tc.want.Title, item.Title)
			}
			if item.URL != tc.want.URL {
				t.Errorf("expected URL %q, got %q", tc.want.URL, item.URL)
			}
		})
	}
}

func TestClient_GetItem_Retries(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(
		WithBaseURL(server.URL),
		WithRetries(3),
		WithRetryWait(0),
	)

	_, err := client.GetItem(3742902)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "failed after 3 attempts") {
		t.Errorf("expected error to contain retry message, got %q", err.Error())
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestClient_GetItem_NetworkError(t *testing.T) {
	client := NewClient(
		WithBaseURL("http://localhost:1"), // invalid port
		WithRetries(1),
		WithRetryWait(0),
	)

	_, err := client.GetItem(3742902)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "request failed") {
		t.Errorf("expected error to contain request failed message, got %q", err.Error())
	}
}

func TestDiscussionURL(t *testing.T) {
	got := DiscussionURL(3742902)
	want := "https://news.ycombinator.com/item?id=3742902"
	if got != want {
		t.Errorf("expected %q, got %q", want, got)
	}
}
