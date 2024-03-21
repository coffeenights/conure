package applications

import "errors"

var (
	ErrDocumentNotFound  = errors.New("document not found")
	ErrDuplicateDocument = errors.New("document already exists")
)
