package apiclient

import (
	"encoding/json"
	"fmt"
	"strings"
)

// ParseServerError extracts a human-readable message from a Conure API error
// response. The server returns JSON shaped like
//
//	{"code":"1004","error":"invalid_credentials"}
//
// (sometimes "message" instead of "error"). When the server attaches a
// "detail" string (an actionable, situation-specific explanation — e.g. which
// credential is missing and the command to create it), it is appended so the
// user sees the fix, not just the bare code. Falling back to the raw body
// keeps non-JSON responses (proxy 502s, blank bodies) from disappearing.
func ParseServerError(body []byte) string {
	trimmed := strings.TrimSpace(string(body))
	if trimmed == "" {
		return "(empty response body)"
	}
	var payload struct {
		Code    string `json:"code"`
		Error   string `json:"error"`
		Message string `json:"message"`
		Detail  string `json:"detail"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return trimmed
	}
	msg := payload.Error
	if msg == "" {
		msg = payload.Message
	}
	if msg == "" {
		return trimmed
	}
	if payload.Code != "" {
		msg = fmt.Sprintf("%s (code %s)", msg, payload.Code)
	}
	if payload.Detail != "" {
		msg = fmt.Sprintf("%s: %s", msg, payload.Detail)
	}
	return msg
}
