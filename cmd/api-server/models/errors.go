package models

import "errors"

var (
	ErrDocumentNotFound  = errors.New("document not found")
	ErrDuplicateDocument = errors.New("document already exists")
)
