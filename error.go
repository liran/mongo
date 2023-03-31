package mongo

import "github.com/pkg/errors"

var (
	ErrInvalidModel       = errors.New("invalid model")
	ErrNotFoundPrimaryKey = errors.New(`not found primary key, defined by tag db:"pk"`)
	ErrNotFoundModel      = errors.New("not found model")
)
