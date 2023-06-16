package mongo

import (
	"context"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Database struct {
	client *Client
	*mongo.Database
}

func (d *Database) Close() {
	if d.client != nil {
		d.client.Close()
		d.client = nil
	}
}

// By default, MongoDB will automatically abort any multi-document transaction that runs for more than 60 seconds.
func (d *Database) Txn(ctx context.Context, fn func(txn *Txn) error, multiDoc ...bool) error {
	if len(multiDoc) > 0 && multiDoc[0] {
		session, err := d.client.StartSession()
		if err != nil {
			return err
		}
		defer session.EndSession(ctx)

		_, err = session.WithTransaction(ctx, func(sc mongo.SessionContext) (any, error) {
			err := fn(&Txn{ctx: sc, db: d})
			return nil, err
		})
		return err
	}

	return fn(&Txn{ctx: ctx, db: d})
}

func (d *Database) Indexes(ctx context.Context, models ...any) error {
	for _, model := range models {
		name, indexes := ParseModelIndex(model)
		if name == "" {
			return ErrInvalidModelName
		}
		if len(indexes) < 1 {
			continue
		}

		var indexModels []mongo.IndexModel
		for indexName, unique := range indexes {
			im := mongo.IndexModel{Keys: bson.D{{Key: indexName, Value: 1}}}
			if unique {
				im.Options = options.Index().SetUnique(true)
			}
			indexModels = append(indexModels, im)
		}
		_, err := d.Collection(name).Indexes().CreateMany(ctx, indexModels)
		if err != nil {
			return err
		}
	}
	return nil
}

func NewDatabase(url string, name string) *Database {
	client := NewClient(url)
	return &Database{client: client, Database: client.Database(name)}
}
