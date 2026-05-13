package apiclient

import "testing"

func TestParseServerError(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{
			name: "conure error envelope with code",
			body: `{"code":"1004","error":"invalid_credentials"}`,
			want: "invalid_credentials (code 1004)",
		},
		{
			name: "envelope using message field instead of error",
			body: `{"code":"2007","message":"invalid_field_value"}`,
			want: "invalid_field_value (code 2007)",
		},
		{
			name: "envelope with no code",
			body: `{"error":"something_broke"}`,
			want: "something_broke",
		},
		{
			name: "JSON object that doesn't match the envelope shape",
			body: `{"unrelated":"thing"}`,
			want: `{"unrelated":"thing"}`,
		},
		{
			name: "malformed JSON falls back to raw body",
			body: `<html>504 Gateway Timeout</html>`,
			want: `<html>504 Gateway Timeout</html>`,
		},
		{
			name: "empty body",
			body: ``,
			want: "(empty response body)",
		},
		{
			name: "whitespace-only body",
			body: "   \n\t  ",
			want: "(empty response body)",
		},
		{
			name: "trims surrounding whitespace on non-JSON fallback",
			body: "  oops  ",
			want: "oops",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseServerError([]byte(tt.body))
			if got != tt.want {
				t.Errorf("ParseServerError(%q) = %q, want %q", tt.body, got, tt.want)
			}
		})
	}
}
