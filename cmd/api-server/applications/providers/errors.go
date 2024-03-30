package providers

import "errors"

var (
	ErrProviderNotSupported = errors.New("provider not supported")
	ErrComponentNotFound    = errors.New("component not found")
	ErrApplicationExists    = errors.New("application already exists")
)
