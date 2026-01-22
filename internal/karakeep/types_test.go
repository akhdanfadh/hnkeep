package karakeep

import "testing"

func TestListBookmarkContent_GetURL(t *testing.T) {
	tests := map[string]struct {
		content ListBookmarkContent
		want    string
	}{
		"link type returns URL": {
			content: ListBookmarkContent{Type: "link", URL: ptr("https://example.com")},
			want:    "https://example.com",
		},
		"link type with nil URL returns empty": {
			content: ListBookmarkContent{Type: "link", URL: nil},
			want:    "",
		},
		"asset type returns sourceUrl": {
			content: ListBookmarkContent{Type: "asset", SourceURL: ptr("https://example.com/doc.pdf")},
			want:    "https://example.com/doc.pdf",
		},
		"asset type with nil sourceUrl returns empty": {
			content: ListBookmarkContent{Type: "asset", SourceURL: nil},
			want:    "",
		},
		"text type returns empty": {
			content: ListBookmarkContent{Type: "text"},
			want:    "",
		},
		"unknown type returns empty": {
			content: ListBookmarkContent{Type: "unknown"},
			want:    "",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got := tc.content.GetURL()
			if got != tc.want {
				t.Errorf("GetURL() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestHTTPError_Error(t *testing.T) {
	err := HTTPError{StatusCode: 500, Body: "internal server error"}
	got := err.Error()
	want := "karakeep API error (HTTP 500): internal server error"
	if got != want {
		t.Errorf("HTTPError.Error() = %q, want %q", got, want)
	}
}

func TestHTTPError_IsClientError(t *testing.T) {
	tests := map[string]struct {
		statusCode int
		want       bool
	}{
		"399 is not client error": {statusCode: 399, want: false},
		"400 is client error":     {statusCode: 400, want: true},
		"404 is client error":     {statusCode: 404, want: true},
		"499 is client error":     {statusCode: 499, want: true},
		"500 is not client error": {statusCode: 500, want: false},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			err := HTTPError{StatusCode: tc.statusCode}
			if got := err.IsClientError(); got != tc.want {
				t.Errorf("HTTPError{%d}.IsClientError() = %v, want %v", tc.statusCode, got, tc.want)
			}
		})
	}
}
