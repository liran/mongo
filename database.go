package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/mongo"
)

type Database struct {
	client *Client
	*mongo.Database
}

func (t *Database) Close() {
	if t.client != nil {
		t.client.Close()
		t.client = nil
	}
}

func (t *Database) Txn(ctx context.Context, fn func(txn *Txn) error, multiDoc ...bool) error {
	if len(multiDoc) > 0 && multiDoc[0] {
		return t.client.UseSession(ctx, func(sc mongo.SessionContext) error {
			return fn(&Txn{ctx: sc, db: t})
		})
	}

	return fn(&Txn{ctx: ctx, db: t})
}

func NewDatabase(url string, name string) *Database {
	client := NewClient(url)
	return &Database{client: client, Database: client.Database(name)}
}
