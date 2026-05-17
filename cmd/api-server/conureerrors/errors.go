package conureerrors

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

type ConureError struct {
	Code       string
	Message    string
	StatusCode int
}

func (e *ConureError) Error() string {
	return fmt.Sprintf("Error %s: %s", e.Code, e.Message)
}

// ConureErrorWithDetail wraps a *ConureError with a human-readable,
// situation-specific explanation. The base error keeps the stable
// code/message enum (clients can still switch on it); Detail carries the
// actionable "here is exactly what went wrong and how to fix it" string that
// otherwise only reached the server log. AbortWithError surfaces Detail in
// the response body so the CLI can show it instead of a bare
// "invalid_request (code 2001)".
type ConureErrorWithDetail struct {
	Base   *ConureError
	Detail string
}

func (e *ConureErrorWithDetail) Error() string {
	if e.Detail == "" {
		return e.Base.Error()
	}
	return fmt.Sprintf("%s: %s", e.Base.Error(), e.Detail)
}

// Unwrap lets errors.As(err, &*ConureError) keep matching, so existing
// callers that branch on the base error are unaffected.
func (e *ConureErrorWithDetail) Unwrap() error { return e.Base }

// WithDetail attaches an actionable explanation to a base ConureError. Use it
// where the handler knows something the fixed message enum can't express
// (e.g. which credential name is missing and the exact command to create it).
func WithDetail(base *ConureError, format string, args ...any) error {
	return &ConureErrorWithDetail{Base: base, Detail: fmt.Sprintf(format, args...)}
}

var (
	ErrUnauthorized              = &ConureError{Code: "1000", Message: "unauthorized", StatusCode: http.StatusUnauthorized}
	ErrInvalidToken              = &ConureError{Code: "1001", Message: "invalid_token", StatusCode: http.StatusUnauthorized}
	ErrJWTKeyError               = &ConureError{Code: "1002", Message: "jwt_key_error", StatusCode: http.StatusInternalServerError}
	ErrCryptoError               = &ConureError{Code: "1003", Message: "crypto_error", StatusCode: http.StatusInternalServerError}
	ErrInvalidCredentials        = &ConureError{Code: "1004", Message: "invalid_credentials", StatusCode: http.StatusUnauthorized}
	ErrOldPasswordInvalid        = &ConureError{Code: "1005", Message: "old_password_invalid", StatusCode: http.StatusBadRequest}
	ErrWrongAuthenticationSystem = &ConureError{Code: "1006", Message: "wrong_authentication_system", StatusCode: http.StatusUnauthorized}
	ErrNotAllowed                = &ConureError{Code: "1007", Message: "not_allowed", StatusCode: http.StatusForbidden}

	ErrInvalidRequest               = &ConureError{Code: "2001", Message: "invalid_request", StatusCode: http.StatusBadRequest}
	ErrObjectNotFound               = &ConureError{Code: "2002", Message: "object_not_found", StatusCode: http.StatusNotFound}
	ErrObjectAlreadyExists          = &ConureError{Code: "2003", Message: "object_already_exists", StatusCode: http.StatusBadRequest}
	ErrInvalidEmail                 = &ConureError{Code: "2004", Message: "invalid_email", StatusCode: http.StatusBadRequest}
	ErrInvalidPassword              = &ConureError{Code: "2005", Message: "invalid_password", StatusCode: http.StatusBadRequest}
	ErrPasswordConfirmationMismatch = &ConureError{Code: "2006", Message: "password_confirmation_mismatch", StatusCode: http.StatusBadRequest}
	ErrFieldValidation              = &ConureError{Code: "2007", Message: "invalid_field_value", StatusCode: http.StatusBadRequest}
	ErrEmailAlreadyExists           = &ConureError{Code: "2008", Message: "email_already_exists", StatusCode: http.StatusBadRequest}

	ErrInternalError = &ConureError{Code: "3001", Message: "internal_error", StatusCode: http.StatusInternalServerError}
	ErrDatabaseError = &ConureError{Code: "3002", Message: "database_error", StatusCode: http.StatusInternalServerError}
	// ErrNetworkError  = &ConureError{Code: "3003", Message: "network_error", StatusCode: http.StatusInternalServerError}

	ErrProviderNotSupported   = &ConureError{Code: "4001", Message: "provider_not_supported", StatusCode: http.StatusInternalServerError}
	ErrComponentNotFound      = &ConureError{Code: "4002", Message: "component_not_found", StatusCode: http.StatusNotFound}
	ErrApplicationExists      = &ConureError{Code: "4003", Message: "application_already_exists", StatusCode: http.StatusConflict}
	ErrApplicationNotDeployed = &ConureError{Code: "4004", Message: "application_not_deployed", StatusCode: http.StatusNotFound}
	ErrPodNotFound            = &ConureError{Code: "4005", Message: "pod_not_found", StatusCode: http.StatusNotFound}
	// ErrAmbiguousComponentEngine is returned when more than one
	// ComponentDefinition matches the requested type and the caller did not
	// pin spec.engine. Mirrors the controller-side error so the API rejects
	// such requests up-front instead of writing a Component that the
	// controller would then fail to render.
	ErrAmbiguousComponentEngine = &ConureError{Code: "4006", Message: "ambiguous_component_engine", StatusCode: http.StatusBadRequest}
	// ErrUnsupportedComponentEngine is returned when the request pins an
	// engine that no ComponentDefinition implements for the requested type.
	ErrUnsupportedComponentEngine = &ConureError{Code: "4007", Message: "unsupported_component_engine", StatusCode: http.StatusBadRequest}
	// ErrBuildLogsNotReady is returned when a build pod exists but its
	// `build` container has not started yet (init containers still running,
	// pod PodInitializing). 425 Too Early signals the client to retry —
	// the CLI's tailRemoteBuild loop polls on this until logs are available.
	ErrBuildLogsNotReady = &ConureError{Code: "4008", Message: "build_logs_not_ready", StatusCode: http.StatusTooEarly}
)

func AbortWithError(c *gin.Context, err error) {
	var detailErr *ConureErrorWithDetail
	var conureErr *ConureError
	var validationErr validator.ValidationErrors

	// Check the detail wrapper first: it Unwraps to *ConureError, so the
	// plain branch below would also match it and silently drop Detail.
	if errors.As(err, &detailErr) {
		c.AbortWithStatusJSON(detailErr.Base.StatusCode, gin.H{
			"code":   detailErr.Base.Code,
			"error":  detailErr.Base.Message,
			"detail": detailErr.Detail,
		})
	} else if errors.As(err, &conureErr) {
		c.AbortWithStatusJSON(conureErr.StatusCode, gin.H{
			"code":  conureErr.Code,
			"error": conureErr.Message,
		})
	} else if errors.As(err, &validationErr) {
		var fieldNames []string
		for _, errorField := range validationErr {
			fieldNames = append(fieldNames, errorField.Field())
		}
		concatenatedErrors := strings.Join(fieldNames, ", ")
		c.AbortWithStatusJSON(http.StatusBadRequest, gin.H{
			"code":    ErrFieldValidation.Code,
			"message": ErrFieldValidation.Message,
			"fields":  concatenatedErrors,
		})
	} else {
		// If the error is not a ConureError, return a generic internal error
		c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{
			"code":  ErrInternalError.Code,
			"error": ErrInternalError.Message,
		})
	}
}
