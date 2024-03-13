package k8s

import "errors"

var (
	ErrApplicationNotFound = errors.New("application not found")
	ErrServiceNotFound     = errors.New("service not found")
)
