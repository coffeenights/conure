package auth

import "errors"

var (
	ErrEmailExists      = errors.New("email already exists")
	ErrEmailNotValid    = errors.New("email is not valid")
	ErrPasswordNotValid = errors.New("password is not valid")
	ErrCryptoHandler    = errors.New("crypto handler error")
	ErrTokenNotValid    = errors.New("token is not valid")
)
