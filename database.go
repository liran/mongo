package mongo

import (
	"context"
	"fmt"
	"strings"

	"go.mongodb.org/mongo-driver/v2/bson"
	driver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Database is a typed facade over an official MongoDB database.
type Database struct {
	*Client
	*driver.Database
}

// Open creates a client and selects a database.
//
// Open validates the client configuration but does not contact the server.
// Call Ping when startup must verify reachability. The caller must call Close.
func Open(uri, name string, configure ...func(*ClientOptions)) (*Database, error) {
	client, err := NewClient(uri, configure...)
	if err != nil {
		return nil, err
	}

	database := &Database{Client: client, Database: client.Database(name)}
	return database, nil
}

// Close disconnects the underlying MongoDB client.
func (d *Database) Close() error {
	if d == nil || d.Client == nil {
		return nil
	}

	err := d.Client.Close()
	d.Client = nil
	return err
}

// Collection returns an explicitly named typed collection bound to ctx.
//
// Use Collection when the collection name is known only at runtime. Normal
// operations derive the name from T or CollectionNamer. Collection panics when
// name is empty because that is a programming error.
//
// Example:
//
//	archive := db.Collection[User](ctx, "archived_users")
//	user, err := archive.Get("user-1")
func (d *Database) Collection[T any](ctx context.Context, name string) *Collection[T] {
	return newCollection[T](d, contextOrBackground(ctx), name)
}

// Save fully replaces a document by _id, inserting it when absent.
//
// Save is not a partial update: fields omitted from document are removed from
// an existing record. Use Update to change selected fields. T is inferred from
// document, whose _id must be tagged with bson:"_id" or db:"pk".
//
// Example:
//
//	user := &User{ID: "user-1", Name: "Liran"}
//	err := db.Save(ctx, user)
func (d *Database) Save[T any](ctx context.Context, document *T) error {
	return defaultCollection[T](d, ctx).Save(document)
}

// SaveMany fully replaces documents by _id using one bulk operation.
//
// Every document is validated for a non-empty _id before MongoDB is called.
// Passing no documents succeeds without issuing a command.
func (d *Database) SaveMany[T any](ctx context.Context, documents ...*T) error {
	return defaultCollection[T](d, ctx).SaveMany(documents...)
}

// Get retrieves a document by _id and decodes it into T.
//
// Get returns an error matching ErrRecordNotFound when the document is absent.
//
// Example:
//
//	user, err := db.Get[User](ctx, "user-1")
func (d *Database) Get[T any](ctx context.Context, id any) (*T, error) {
	return defaultCollection[T](d, ctx).Get(id)
}

// Update applies $set to selected fields and returns the resulting document.
//
// Unlike struct-based patching, zero values in fields are written. Any _id in
// fields is ignored. A missing document returns ErrRecordNotFound.
//
// Example:
//
//	fields := mongo.M{"active": false, "name": "new name"}
//	user, err := db.Update[User](ctx, "user-1", fields)
func (d *Database) Update[T any](ctx context.Context, id, fields any) (*T, error) {
	return defaultCollection[T](d, ctx).Update(id, fields)
}

// Increment atomically applies $inc and returns the resulting document.
// A missing document returns ErrRecordNotFound.
func (d *Database) Increment[T any](ctx context.Context, id, fields any) (*T, error) {
	return defaultCollection[T](d, ctx).Increment(id, fields)
}

// Delete removes a document by _id. Deleting an absent document succeeds.
func (d *Database) Delete[T any](ctx context.Context, id any) error {
	return defaultCollection[T](d, ctx).Delete(id)
}

// UpdateMany applies $set to every document matching a non-empty filter.
//
// It returns MongoDB's modified count, which can be smaller than the matched
// count when documents already contain the requested values. Empty filters
// return ErrEmptyFilter to prevent accidental collection-wide writes.
func (d *Database) UpdateMany[T any](ctx context.Context, filter, fields any) (int64, error) {
	return defaultCollection[T](d, ctx).UpdateMany(filter, fields)
}

// DeleteMany removes every document matching a non-empty filter.
// It returns ErrEmptyFilter without contacting MongoDB for an empty filter.
func (d *Database) DeleteMany[T any](ctx context.Context, filter any) (int64, error) {
	return defaultCollection[T](d, ctx).DeleteMany(filter)
}

// Find starts a reusable typed query bound to ctx.
//
// Pass zero or one filter. Omitting it matches every document. Chain methods
// return independent query values, so a base query can be safely reused.
//
// Example:
//
//	filter := mongo.M{"active": true}
//	users, err := db.Find[User](ctx, filter).Sort(mongo.M{"score": -1}).All()
func (d *Database) Find[T any](ctx context.Context, filter ...any) *Query[T] {
	return defaultCollection[T](d, ctx).Find(filter...)
}

// EnsureIndexes creates indexes declared by db struct tags on T.
//
// Existing indexes with the same ordered keys are left unchanged. This method
// creates missing indexes; it does not alter or remove existing definitions.
//
// Example:
//
//	err := db.EnsureIndexes[User](ctx)
func (d *Database) EnsureIndexes[T any](ctx context.Context) error {
	document := new(T)
	_, indexInfo := ParseModelIndexes(document)
	return d.createIndexes(contextOrBackground(ctx), modelName[T](), indexInfo)
}

func (d *Database) createIndexes(ctx context.Context, name string, indexInfo map[string]*CompoundIndex) error {
	if name == "" {
		return ErrInvalidModelName
	}
	if len(indexInfo) == 0 {
		return nil
	}

	indexView := d.Database.Collection(name).Indexes()
	specifications, err := indexView.ListSpecifications(ctx)
	if err != nil {
		return err
	}

	existingKeys := make(map[string]struct{}, len(specifications))
	for _, specification := range specifications {
		keys := bson.D{}
		if err := bson.Unmarshal(specification.KeysDocument, &keys); err != nil {
			return err
		}
		existingKeys[keysToString(keys)] = struct{}{}
	}

	for groupName, info := range indexInfo {
		if len(info.Fields) == 0 {
			continue
		}

		keys := make(bson.D, 0, len(info.Fields))
		for _, fieldName := range info.Fields {
			key := bson.E{Key: fieldName, Value: 1}
			keys = append(keys, key)
		}
		keyString := keysToString(keys)
		if _, exists := existingKeys[keyString]; exists {
			continue
		}

		indexOptions := options.Index().SetUnique(info.Unique)
		if len(info.Fields) > 1 {
			indexOptions.SetName(groupName)
		}
		indexModel := driver.IndexModel{Keys: keys, Options: indexOptions}
		if _, err := indexView.CreateOne(ctx, indexModel); err != nil {
			return err
		}
		existingKeys[keyString] = struct{}{}
	}

	return nil
}

func defaultCollection[T any](database *Database, ctx context.Context) *Collection[T] {
	return newCollection[T](database, contextOrBackground(ctx), modelName[T]())
}

func contextOrBackground(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func keysToString(keys bson.D) string {
	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key.Key+":"+fmt.Sprintf("%v", key.Value))
	}
	return strings.Join(parts, ",")
}
