// Package mongo defines common error types for MongoDB operations.
package mongo

import "errors"

var (
	// ErrInvalidModelName is returned when a model name cannot be determined.
	ErrInvalidModelName = errors.New("invalid model name")

	// ErrNoID is returned when a model does not have a valid primary key.
	// The primary key must be marked with bson:"_id" or db:"pk" tag.
	ErrNoID = errors.New(`no id. not found primary key from model, defined by tag db:"pk" or bson:"_id"`)

	// ErrRecordNotFound is returned when a requested record does not exist.
	ErrRecordNotFound = errors.New("record not found")

	// ErrDuplicateKey is returned when a unique constraint violation occurs.
	ErrDuplicateKey = errors.New("duplicate key error")

	// ErrEmptyFilter prevents accidental collection-wide updates and deletes.
	ErrEmptyFilter = errors.New("empty filter is not allowed for bulk mutation")
)
