package apiclient

import (
	"strings"
	"testing"
)

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
		{
			// The fix: a server-supplied detail is appended after the code so
			// the user sees the actionable reason, not just the bare code.
			name: "detail appended after code",
			body: `{"code":"2001","error":"invalid_request","detail":"create it with ` + "`conure credential set git-trivor --kind git ...`" + `"}`,
			want: "invalid_request (code 2001): create it with `conure credential set git-trivor --kind git ...`",
		},
		{
			name: "detail appended without code",
			body: `{"error":"invalid_request","detail":"the why"}`,
			want: "invalid_request: the why",
		},
		{
			name: "empty detail not appended",
			body: `{"code":"2002","error":"object_not_found","detail":""}`,
			want: "object_not_found (code 2002)",
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

// The detail must be the trailing segment so it reads as a sentence after
// the code, and the raw credential-fix command must survive verbatim (the
// user copy-pastes it).
func TestParseServerError_DetailIsActionableAndLast(t *testing.T) {
	body := `{"code":"2001","error":"invalid_request","detail":"create it with ` + "`conure credential set git-trivor --kind git ...`" + `"}`
	got := ParseServerError([]byte(body))
	if !strings.HasSuffix(got, "`conure credential set git-trivor --kind git ...`") {
		t.Fatalf("detail (with fix command) must be the trailing segment, got %q", got)
	}
}
