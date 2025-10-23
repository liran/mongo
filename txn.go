// Package mongo provides transaction management for MongoDB operations.
package mongo

import (
	"context"
)

// Txn represents a database transaction context.
// It provides methods for performing operations within a transaction.
type Txn struct {
	ctx context.Context
	db  *Database
}

// Model creates a new Model instance for the given model type within this transaction.
// All operations performed on the returned Model will be part of this transaction.
func (txn *Txn) Model(model any) *Model {
	return NewModel(txn, model)
}
