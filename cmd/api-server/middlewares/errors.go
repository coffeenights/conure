package middlewares

import (
	"errors"
)

var (
	ErrUnsupportedStrategy = errors.New("unsupported authentication system")
)
