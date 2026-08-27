package mongo

import (
	"context"
	"fmt"

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
	return newCollection[T](d, ctx, name)
}

// Save fully replaces a document by _id, inserting it when absent.
//
// Save is not a partial update: fields omitted from document are removed from
// an existing record. Use Update to change selected fields. T is inferred from
// document, whose _id must be tagged with bson:"_id".
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
// Existing indexes must satisfy the declared keys, custom name, and unique
// option. Conflicting definitions return ErrIndexConflict. This method creates
// missing indexes but never alters or removes existing indexes.
//
// Example:
//
//	err := db.EnsureIndexes[User](ctx)
func (d *Database) EnsureIndexes[T any](ctx context.Context) error {
	definitions, err := IndexesFor[T]()
	if err != nil {
		return err
	}
	return d.createIndexes(ctx, CollectionName[T](), definitions)
}

func (d *Database) createIndexes(ctx context.Context, name string, definitions []IndexDefinition) error {
	if name == "" {
		return ErrInvalidModelName
	}
	if len(definitions) == 0 {
		return nil
	}

	indexView := d.Database.Collection(name).Indexes()
	specifications, err := indexView.ListSpecifications(ctx)
	if err != nil {
		return err
	}

	existingIndexes := make([]existingIndex, 0, len(specifications))
	for _, specification := range specifications {
		keys := bson.D{}
		if err := bson.Unmarshal(specification.KeysDocument, &keys); err != nil {
			return err
		}
		unique := specification.Unique != nil && *specification.Unique
		existing := existingIndex{Name: specification.Name, Keys: keys, Unique: unique}
		existingIndexes = append(existingIndexes, existing)
	}

	for _, definition := range definitions {
		satisfied, conflict := matchExistingIndex(definition, existingIndexes)
		if satisfied {
			continue
		}
		if conflict != "" {
			return fmt.Errorf("%w: %s", ErrIndexConflict, conflict)
		}

		indexOptions := options.Index().SetUnique(definition.Unique)
		if definition.Name != "" {
			indexOptions.SetName(definition.Name)
		}
		indexModel := driver.IndexModel{Keys: definition.Keys, Options: indexOptions}
		if _, err := indexView.CreateOne(ctx, indexModel); err != nil {
			return err
		}
		created := existingIndex(definition)
		existingIndexes = append(existingIndexes, created)
	}

	return nil
}

type existingIndex struct {
	Name   string
	Keys   bson.D
	Unique bool
}

func matchExistingIndex(definition IndexDefinition, indexes []existingIndex) (bool, string) {
	conflict := ""
	for _, index := range indexes {
		if definition.Name != "" && definition.Name == index.Name && !indexKeysEqual(definition.Keys, index.Keys) {
			return false, fmt.Sprintf("name %q already uses different keys", definition.Name)
		}
		if !indexKeysEqual(definition.Keys, index.Keys) {
			continue
		}

		nameMatches := definition.Name == "" || definition.Name == index.Name
		uniqueMatches := !definition.Unique || index.Unique
		if nameMatches && uniqueMatches {
			return true, ""
		}
		if definition.Name != "" && !nameMatches {
			conflict = fmt.Sprintf("keys %v already exist as %q instead of %q", definition.Keys, index.Name, definition.Name)
			continue
		}
		if definition.Unique && !index.Unique {
			conflict = fmt.Sprintf("keys %v already exist without unique=true", definition.Keys)
		}
	}
	return false, conflict
}

func defaultCollection[T any](database *Database, ctx context.Context) *Collection[T] {
	return newCollection[T](database, ctx, CollectionName[T]())
}
