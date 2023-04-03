package mongo

import "github.com/pkg/errors"

var (
	ErrInvalidModelName = errors.New("invalid model name")
	ErrNoID             = errors.New(`no id. not found primary key from model, defined by tag db:"pk" or bson:"_id"`)
	ErrNotFoundModel    = errors.New("not found model")
)
