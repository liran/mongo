// Package mongo provides database operations and transaction management.
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

// Database represents a MongoDB database connection with enhanced operations.
// It provides high-level methods for CRUD operations, transactions, and index management.
type Database struct {
	*Client
	*mongo.Database
}

// NewDatabase creates a new database connection with the specified URL and database name.
// Optional client options can be provided to customize the connection behavior.
//
// Example:
//
//	db := mongo.NewDatabase("mongodb://localhost:27017", "myapp")
//	db := mongo.NewDatabase(uri, "myapp", func(c *mongo.ClientOptions) {
//	    c.SetMaxPoolSize(100)
//	})
func NewDatabase(url string, name string, opts ...func(c *ClientOptions)) *Database {
	client := NewClient(url, opts...)
	return &Database{Client: client, Database: client.Database(name)}
}

// Close closes the database connection and cleans up resources.
// It safely handles nil clients to prevent panics.
func (d *Database) Close() {
	if d.Client != nil {
		d.Client.Close()
		d.Client = nil
	}
}

// Txn executes a transaction with the given function. By default, MongoDB will automatically abort any multi-document transaction that runs for more than 60 seconds.
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

// Indexes creates indexes for the given models based on their struct tags.
// It supports both single and compound indexes with automatic naming.
// Duplicate indexes are automatically skipped.
//
// Example:
//
//	err := db.Indexes(ctx, &User{}, &Product{})
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

// Set creates or updates a record in the database (upsert operation).
// The record must have a valid ID field marked with bson:"_id" or db:"pk".
//
// Example:
//
//	user := &User{ID: "user123", Name: "John", Email: "john@example.com"}
//	err := db.Set(user)
//	if err != nil {
//	    log.Fatal(err)
//	}
func (d *Database) Set(record any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return d.Txn(ctx, func(txn *Txn) error {
		return txn.Model(record).Set(record)
	})
}

// Delete removes a record from the database by its ID.
//
// Example:
//
//	err := db.Delete(&User{}, "user123")
func (d *Database) Delete(model any, id string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return d.Txn(ctx, func(txn *Txn) error {
		return txn.Model(model).Del(id)
	})
}

// Update updates an existing record and returns the updated document.
// Returns ErrRecordNotFound if the record doesn't exist.
//
// Example:
//
//	user.Name = "Jane"
//	user.Age = 31
//	updated, err := db.Update(user)
//	if err != nil {
//	    if errors.Is(err, mongo.ErrRecordNotFound) {
//	        log.Println("User not found")
//	    } else {
//	        log.Fatal(err)
//	    }
//	}
func (d *Database) Update(record any) (newRecord M, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	d.Txn(ctx, func(txn *Txn) error {
		newRecord, err = txn.Model(record).Update(record)
		return err
	})

	return
}

// Pagination retrieves paginated results with total count.
// Supports filtering, sorting, and field projection.
//
// Example:
//
//	filter := mongo.Map().Set("age", mongo.Map().Set("$gte", 18))
//	sort := mongo.Map().Set("created_at", -1)
//	total, users, err := db.Pagination(&User{}, filter, sort, 1, 10)
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Found %d users, showing page 1\n", total)
func (d *Database) Pagination(model, filter, sort any, page, pageSize int64, projection ...any) (total int64, list []M, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	d.Txn(ctx, func(txn *Txn) error {
		total, list, err = txn.Model(model).Pagination(filter, sort, page, pageSize, projection...)
		return err
	})

	return
}

// Unmarshal retrieves a record by ID and unmarshals it into the provided model.
// Returns ErrRecordNotFound if the record doesn't exist.
//
// Example:
//
//	var user User
//	err := db.Unmarshal("user123", &user)
func (d *Database) Unmarshal(id, model any, projection ...any) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return d.Txn(ctx, func(txn *Txn) error {
		return txn.Model(model).Unmarshal(id, model, projection...)
	})
}

// First retrieves the first record matching the filter criteria.
// Supports sorting and field projection.
//
// Example:
//
//	record, err := db.First(&User{}, filter, sort)
func (d *Database) First(model, filter, sort any, projection ...any) (record M, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	d.Txn(ctx, func(txn *Txn) error {
		record, err = txn.Model(model).First(filter, sort, projection...)
		return err
	})

	return
}

// Count returns the number of documents matching the filter.
// If filter is nil or empty, returns the total document count.
//
// Example:
//
//	count, err := db.Count(&User{}, filter)
func (d *Database) Count(model, filter any) (count int64, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	d.Txn(ctx, func(txn *Txn) error {
		count, err = txn.Model(model).Count(filter)
		return err
	})

	return
}

// Has checks if a record with the given ID exists in the database.
//
// Example:
//
//	exists, err := db.Has(&User{}, "user123")
func (d *Database) Has(model, id any) (exists bool, err error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	d.Txn(ctx, func(txn *Txn) error {
		exists, err = txn.Model(model).Has(id)
		return err
	})

	return
}

// List iterates over documents matching the filter using a callback function.
// The callback can return false to stop iteration early.
//
// Example:
//
//	filter := mongo.Map().Set("status", "active")
//	err := db.List(ctx, &User{}, filter, func(user M) (bool, error) {
//	    fmt.Printf("User: %s\n", user.Get("name"))
//	    return true, nil // continue iteration
//	})
func (d *Database) List(ctx context.Context, model any, filter M, cb func(m M) (bool, error), projection ...any) error {
	return d.Txn(ctx, func(txn *Txn) error {
		return txn.Model(model).List(filter, cb, projection...)
	})
}

// keysToString converts bson.D to string for index key comparison.
// Used internally for checking if indexes already exist.
func keysToString(keys bson.D) string {
	var parts []string
	for _, key := range keys {
		parts = append(parts, key.Key+":"+fmt.Sprintf("%v", key.Value))
	}
	return strings.Join(parts, ",")
}

// keysMapToString converts bson.M to string for index key comparison.
// Used internally for checking if indexes already exist.
func keysMapToString(keys bson.M) string {
	var parts []string
	for key, value := range keys {
		parts = append(parts, key+":"+fmt.Sprintf("%v", value))
	}
	return strings.Join(parts, ",")
}
