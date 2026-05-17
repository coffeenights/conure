package conureerrors

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

// WithDetail must wrap, not replace: errors.As(&*ConureError) still matches
// (handlers that branch on the base error keep working), and Unwrap exposes
// the same base instance.
func TestWithDetail_UnwrapsToBase(t *testing.T) {
	err := WithDetail(ErrInvalidRequest, "credential %q missing", "git-trivor")

	var base *ConureError
	if !errors.As(err, &base) {
		t.Fatal("errors.As(&*ConureError) must still match a detail-wrapped error")
	}
	if base != ErrInvalidRequest {
		t.Fatalf("Unwrap should expose the original base, got %+v", base)
	}
	if got := err.Error(); got != `Error 2001: invalid_request: credential "git-trivor" missing` {
		t.Fatalf("Error() = %q", got)
	}
}

func TestAbortWithError_Envelopes(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("plain ConureError has no detail key", func(t *testing.T) {
		c, w := testCtx()
		AbortWithError(c, ErrObjectNotFound)

		if w.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404", w.Code)
		}
		body := decode(t, w)
		if body["code"] != "2002" || body["error"] != "object_not_found" {
			t.Fatalf("unexpected envelope: %v", body)
		}
		if _, ok := body["detail"]; ok {
			t.Fatalf("plain error must not emit a detail key: %v", body)
		}
	})

	t.Run("WithDetail surfaces detail and keeps base code/status", func(t *testing.T) {
		c, w := testCtx()
		AbortWithError(c, WithDetail(ErrInvalidRequest,
			"component references git credential %q which this organization has not loaded", "git-trivor"))

		// Base status/code preserved so existing clients still switch on it.
		if w.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (base status preserved)", w.Code)
		}
		body := decode(t, w)
		if body["code"] != "2001" || body["error"] != "invalid_request" {
			t.Fatalf("base code/message must be preserved: %v", body)
		}
		want := `component references git credential "git-trivor" which this organization has not loaded`
		if body["detail"] != want {
			t.Fatalf("detail = %q, want %q", body["detail"], want)
		}
	})
}

func testCtx() (*gin.Context, *httptest.ResponseRecorder) {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/", nil)
	return c, w
}

func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("response body is not JSON (%v): %s", err, w.Body.String())
	}
	return m
}
