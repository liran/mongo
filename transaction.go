package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

// Tx is a typed operation scope inside a MongoDB transaction.
type Tx struct {
	ctx      context.Context
	database *Database
}

// Transaction executes fn in a real MongoDB multi-document transaction.
//
// All Tx operations use the transaction's session context. The driver may run
// fn more than once for retryable errors, so fn must be idempotent and must not
// hide operation errors. Returning an error aborts the transaction.
//
// Example:
//
//	err := db.Transaction(ctx, func(tx *mongo.Tx) error {
//		if err := tx.Save(user); err != nil {
//			return err
//		}
//		_, err := tx.Update[Account](accountID, fields)
//		return err
//	})
func (d *Database) Transaction(ctx context.Context, fn func(*Tx) error) error {
	transactionOptions := options.Transaction().SetReadPreference(readpref.Primary())
	sessionOptions := options.Session().SetDefaultTransactionOptions(transactionOptions)
	session, err := d.Client.StartSession(sessionOptions)
	if err != nil {
		return err
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(sessionCtx context.Context) (any, error) {
		transaction := &Tx{ctx: sessionCtx, database: d}
		return nil, fn(transaction)
	})
	return err
}

// Collection returns an explicitly named typed collection in the transaction.
func (tx *Tx) Collection[T any](name string) *Collection[T] {
	return newCollection[T](tx.database, tx.ctx, name)
}

// Save fully replaces a document by _id inside the transaction.
func (tx *Tx) Save[T any](document *T) error {
	return txCollection[T](tx).Save(document)
}

// SaveMany fully replaces documents by _id using one transactional bulk command.
func (tx *Tx) SaveMany[T any](documents ...*T) error {
	return txCollection[T](tx).SaveMany(documents...)
}

// Get retrieves a document by _id inside the transaction.
func (tx *Tx) Get[T any](id any) (*T, error) {
	return txCollection[T](tx).Get(id)
}

// Update applies $set and returns the resulting document inside the transaction.
func (tx *Tx) Update[T any](id, fields any) (*T, error) {
	return txCollection[T](tx).Update(id, fields)
}

// Increment atomically applies $inc inside the transaction.
func (tx *Tx) Increment[T any](id, fields any) (*T, error) {
	return txCollection[T](tx).Increment(id, fields)
}

// Delete removes a document by _id inside the transaction.
func (tx *Tx) Delete[T any](id any) error {
	return txCollection[T](tx).Delete(id)
}

// UpdateMany applies $set to documents matching a non-empty filter.
func (tx *Tx) UpdateMany[T any](filter, fields any) (int64, error) {
	return txCollection[T](tx).UpdateMany(filter, fields)
}

// DeleteMany removes documents matching a non-empty filter.
func (tx *Tx) DeleteMany[T any](filter any) (int64, error) {
	return txCollection[T](tx).DeleteMany(filter)
}

// Find starts a typed query that participates in the transaction.
func (tx *Tx) Find[T any](filter ...any) *Query[T] {
	return txCollection[T](tx).Find(filter...)
}

func txCollection[T any](tx *Tx) *Collection[T] {
	return newCollection[T](tx.database, tx.ctx, CollectionName[T]())
}
