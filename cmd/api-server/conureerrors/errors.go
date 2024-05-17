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
)

func AbortWithError(c *gin.Context, err error) {
	var conureErr *ConureError
	var validationErr validator.ValidationErrors

	if errors.As(err, &conureErr) {
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
