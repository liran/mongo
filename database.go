package mongo

import (
	"context"
	"fmt"
	"strings"
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
		name, indexInfo := ParseModelIndexes(model)
		if name == "" {
			return ErrInvalidModelName
		}
		if len(indexInfo) == 0 {
			continue
		}

		collection := d.Collection(name)
		indexView := collection.Indexes()

		// Get existing indexes
		existingIndexes, err := indexView.List(ctx)
		if err != nil {
			return err
		}

		// Create a map of existing index keys for quick lookup
		existingIndexKeys := make(map[string]struct{})
		for existingIndexes.Next(ctx) {
			var indexDoc bson.M
			if err := existingIndexes.Decode(&indexDoc); err != nil {
				return err
			}
			if keys, ok := indexDoc["key"].(bson.M); ok {
				// Convert keys to a string representation for comparison
				keyStr := keysMapToString(keys)
				existingIndexKeys[keyStr] = struct{}{}
			}
		}
		existingIndexes.Close(ctx)

		// Create indexes that don't exist
		for groupName, v := range indexInfo {
			if len(v.Fields) == 0 {
				continue
			}

			// Create compound index keys
			keys := bson.D{}
			for _, fieldName := range v.Fields {
				keys = append(keys, bson.E{Key: fieldName, Value: 1})
			}

			// Check if this index already exists
			keyStr := keysToString(keys)
			if _, ok := existingIndexKeys[keyStr]; ok {
				continue // Skip if index already exists
			}

			im := mongo.IndexModel{Keys: keys}
			im.Options = options.Index().SetUnique(v.Unique)
			if len(v.Fields) > 1 {
				im.Options.SetName(groupName)
			}

			// Create the index
			_, err := indexView.CreateOne(ctx, im)
			if err != nil {
				return err
			}
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

// keysToString converts bson.D to string for index key comparison
func keysToString(keys bson.D) string {
	var parts []string
	for _, key := range keys {
		parts = append(parts, key.Key+":"+fmt.Sprintf("%v", key.Value))
	}
	return strings.Join(parts, ",")
}

// keysMapToString converts bson.M to string for index key comparison
func keysMapToString(keys bson.M) string {
	var parts []string
	for key, value := range keys {
		parts = append(parts, key+":"+fmt.Sprintf("%v", value))
	}
	return strings.Join(parts, ",")
}
