package apiclient

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// APIError is a non-2xx response from the Conure API. It carries the HTTP
// status and the server's machine-readable error code (e.g. "2002" for
// object_not_found) so callers can branch on a specific failure instead of
// string-matching the message. The Error() string is the same human-readable
// rendering ParseServerError produces, so existing error output is unchanged.
type APIError struct {
	StatusCode int
	Code       string // server error code, e.g. "2002"; "" if the body had none
	Message    string // human-readable rendering (ParseServerError output)
}

func (e *APIError) Error() string {
	return fmt.Sprintf("server returned HTTP %d: %s", e.StatusCode, e.Message)
}

// codeObjectNotFound is conureerrors.ErrObjectNotFound's code. Duplicated here
// rather than imported: apiclient deliberately depends only on pkg/api and the
// stdlib (see package doc), and the server's error codes are a stable wire
// contract.
const codeObjectNotFound = "2002"

// IsObjectNotFound reports whether err is an APIError for a missing object
// (HTTP 404, code 2002). Callers use it to offer a fallback instead of
// surfacing a bare "object_not_found".
func IsObjectNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Code == codeObjectNotFound
}

// newAPIError builds an APIError from a non-2xx response. The Message field
// is ParseServerError's rendering; Code is the server's machine-readable code
// (empty when the body carried none, e.g. a proxy 502).
func newAPIError(statusCode int, body []byte) *APIError {
	return &APIError{
		StatusCode: statusCode,
		Code:       parseServerErrorCode(body),
		Message:    ParseServerError(body),
	}
}

// parseServerErrorCode pulls just the "code" field out of a Conure error body.
// Returns "" for non-JSON or code-less bodies.
func parseServerErrorCode(body []byte) string {
	var payload struct {
		Code string `json:"code"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return ""
	}
	return payload.Code
}

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
