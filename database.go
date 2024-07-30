package mongo

import (
	"context"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type Database struct {
	*Client
	*mongo.Database
}

func NewDatabase(url string, name string, opts ...func(c *ClientOptions)) *Database {
	client := NewClient(url, opts...)
	return &Database{Client: client, Database: client.Database(name)}
}

func (d *Database) Close() {
	if d.Client != nil {
		d.Client.Close()
		d.Client = nil
	}
}

// By default, MongoDB will automatically abort any multi-document transaction that runs for more than 60 seconds.
func (d *Database) Txn(ctx context.Context, fn func(txn *Txn) error, multiDoc ...bool) error {
	if len(multiDoc) > 0 && multiDoc[0] {
		// read preference in a transaction must be primary
		sesstionOptions := &options.SessionOptions{DefaultReadPreference: readpref.Primary()}
		session, err := d.Client.StartSession(sesstionOptions)
		if err != nil {
			return err
		}
		defer session.EndSession(ctx)

		_, err = session.WithTransaction(ctx, func(sc mongo.SessionContext) (any, error) {
			return nil, fn(&Txn{ctx: sc, db: d})
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

func (o *Database) Set(record any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return o.Txn(ctx, func(txn *Txn) error {
		return txn.Model(record).Set(record)
	})
}

func (o *Database) Delete(model any, id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return o.Txn(ctx, func(txn *Txn) error {
		return txn.Model(model).Del(id)
	})
}

func (o *Database) Update(record any) (newRecord M, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	o.Txn(ctx, func(txn *Txn) error {
		newRecord, err = txn.Model(record).Update(record)
		return err
	})

	return
}

func (o *Database) Pagination(model, filter, sort any, page, pageSize int64, projection ...any) (total int64, list []M, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	o.Txn(ctx, func(txn *Txn) error {
		total, list, err = txn.Model(model).Pagination(filter, sort, page, pageSize, projection...)
		return err
	})

	return
}

func (o *Database) Unmarshal(id, model any, projection ...any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return o.Txn(ctx, func(txn *Txn) error {
		return txn.Model(model).Unmarshal(id, model, projection...)
	})
}

func (o *Database) First(model, filter, sort any, projection ...any) (record M, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	o.Txn(ctx, func(txn *Txn) error {
		record, err = txn.Model(model).First(filter, sort, projection...)
		return err
	})

	return
}

func (o *Database) Count(model, filter any) (count int64, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	o.Txn(ctx, func(txn *Txn) error {
		count, err = txn.Model(model).Count(filter)
		return err
	})

	return
}

func (o *Database) Has(model, id any) (exists bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	o.Txn(ctx, func(txn *Txn) error {
		exists, err = txn.Model(model).Has(id)
		return err
	})

	return
}

func (o *Database) List(ctx context.Context, model any, filter M, cb func(m M) (bool, error), projection ...any) error {
	return o.Txn(ctx, func(txn *Txn) error {
		return txn.Model(model).List(filter, cb, projection...)
	})
}
