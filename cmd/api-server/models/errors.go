package models

import "errors"

var (
	ErrDocumentNotFound  = errors.New("document not found")
	ErrDuplicateDocument = errors.New("document already exists")

	// Auth errors
	ErrEmailExists       = errors.New("email already exists")
	ErrEmailNotValid     = errors.New("email is not valid")
	ErrPasswordNotValid  = errors.New("password is not valid")
	ErrPasswordsNotMatch = errors.New("password and password confirmation do not match")
)
