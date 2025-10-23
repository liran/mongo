// Package mongo provides transaction management for MongoDB operations.
package mongo

import (
	"context"
)

type Txn struct {
	ctx context.Context
	db  *Database
}

func (txn *Txn) Model(model any) *Model {
	return NewModel(txn, model)
}
