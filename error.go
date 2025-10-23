// Package mongo defines common error types for MongoDB operations.
package mongo

import (
	"strings"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/mongo"
)

var (
	ErrInvalidModelName = errors.New("invalid model name")
	ErrNoID             = errors.New(`no id. not found primary key from model, defined by tag db:"pk" or bson:"_id"`)
	ErrRecordNotFound   = errors.New("record not found")
	ErrDuplicateKey     = errors.New("duplicate key error")
)

// isDuplicateKeyError checks if the error is a MongoDB duplicate key error
func isDuplicateKeyError(err error) bool {
	if err == nil {
		return false
	}

	// Check for MongoDB duplicate key error code E11000
	if writeErr, ok := err.(mongo.WriteException); ok {
		for _, we := range writeErr.WriteErrors {
			if we.Code == 11000 {
				return true
			}
		}
	}

	// Check for bulk write duplicate key error
	if bulkErr, ok := err.(mongo.BulkWriteException); ok {
		for _, we := range bulkErr.WriteErrors {
			if we.Code == 11000 {
				return true
			}
		}
	}

	// Check error message for duplicate key pattern
	errMsg := strings.ToLower(err.Error())
	return strings.Contains(errMsg, "duplicate key") ||
		strings.Contains(errMsg, "e11000") ||
		strings.Contains(errMsg, "index:") && strings.Contains(errMsg, "dup key")
}
