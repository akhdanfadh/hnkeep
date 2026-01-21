package karakeep

import "testing"

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
