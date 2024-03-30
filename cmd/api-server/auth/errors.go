package auth

import "errors"

var (
	ErrCryptoHandler      = errors.New("crypto handler error")
	ErrTokenNotValid      = errors.New("token is not valid")
	ErrEmailPasswordValid = errors.New("invalid email or password")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrOldPasswordInvalid = errors.New("old password is invalid")
	ErrJWTSecretKey       = errors.New("jwt secret key error")
)
