package auth

import "errors"

var (
	ErrEmailExists        = errors.New("email already exists")
	ErrEmailNotValid      = errors.New("email is not valid")
	ErrPasswordNotValid   = errors.New("password is not valid")
	ErrCryptoHandler      = errors.New("crypto handler error")
	ErrTokenNotValid      = errors.New("token is not valid")
	ErrEmailPasswordValid = errors.New("invalid email or password")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrPasswordsNotMatch  = errors.New("password and password confirmation do not match")
	ErrOldPasswordInvalid = errors.New("old password is invalid")
)
