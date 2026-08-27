# MongoDB SDK for Go 1.27

A small typed layer over MongoDB Go Driver v2. Every operation accepts the caller's `context.Context`; reads return application types directly.

## Requirements

- Go 1.27+
- MongoDB 4.2+
- A replica set when using multi-document transactions

## Quick start

```go
package main

import (
	"context"
	"log"

	"github.com/liran/mongo"
)

type User struct {
	ID     string `bson:"_id"`
	Email  string `bson:"email" db:"unique"`
	Name   string `bson:"name,omitempty"`
	Active bool   `bson:"active" db:"index"`
}

func main() {
	ctx := context.Background()
	db, err := mongo.Open("mongodb://localhost:27017", "myapp")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := db.EnsureIndexes[User](ctx); err != nil {
		log.Fatal(err)
	}

	user := &User{ID: "user-1", Email: "liran@example.com", Name: "Liran"}
	if err := db.Save(ctx, user); err != nil {
		log.Fatal(err)
	}

	user, err = db.Get[User](ctx, user.ID)
	if err != nil {
		log.Fatal(err)
	}
}
```

`Save` is a full-document replacement with upsert semantics. The document must contain a field tagged `bson:"_id"` or `db:"pk"`.

## CRUD

```go
err := db.Save(ctx, user)
err = db.SaveMany(ctx, user1, user2)

user, err = db.Get[User](ctx, id)
user, err = db.Update[User](ctx, id, mongo.M{"name": "new name", "active": false})
user, err = db.Increment[User](ctx, id, mongo.M{"score": 1})

err = db.Delete[User](ctx, id)
modified, err := db.UpdateMany[User](ctx, filter, fields)
deleted, err := db.DeleteMany[User](ctx, filter)
```

`Update` and `UpdateMany` apply `$set`, so zero values are written normally and `_id` is ignored. Empty filters are rejected for bulk updates and deletes.

## Queries

```go
active := mongo.M{"active": true}

users, err := db.Find[User](ctx, active).
	Sort(mongo.M{"created_at": -1}).
	Limit(20).
	All()

user, err := db.Find[User](ctx, mongo.M{"email": email}).First()
exists, err := db.Find[User](ctx, mongo.M{"email": email}).Exists()
count, err := db.Find[User](ctx, active).Count()
```

Projection is part of the same chain:

```go
users, err := db.Find[User](ctx, active).
	Select(mongo.M{"_id": 1, "name": 1}).
	All()
```

Offset pagination returns one named result instead of parallel return values:

```go
page, err := db.Find[User](ctx, active).
	Sort(mongo.M{"created_at": -1}).
	Page(1, 20)

log.Printf("%d total, %d returned", page.Total, len(page.Items))
```

For stable large-result pagination, use `_id` cursors:

```go
next, err := db.Find[User](ctx, active).After(lastID).Limit(20).All()
previous, err := db.Find[User](ctx, active).After(lastID).Desc().Limit(20).All()
```

`Each` traverses large result sets in bounded `_id` batches:

```go
err := db.Find[User](ctx, active).Batch(500).Each(
	func(user *User) (bool, error) {
		return true, process(user)
	},
)
```

Return `false` to stop without an error.

## Transactions

Normal calls already use the supplied context; do not open a transaction just to bind `ctx`. Use `Transaction` only for an atomic multi-document change:

```go
err := db.Transaction(ctx, func(tx *mongo.Tx) error {
	if err := tx.Save(user); err != nil {
		return err
	}

	fields := mongo.M{"balance": 0}
	_, err := tx.Update[Account](accountID, fields)
	return err
})
```

MongoDB may retry the callback, so it must be idempotent and return every operation error.

## Collection names

`UserProfile` maps to `user_profile` by default. Override it on the model when needed:

```go
func (*User) CollectionName() string {
	return "users"
}
```

For a runtime collection name with a known schema:

```go
archive := db.Collection[User](ctx, "archived_users")
user, err := archive.Get(id)
```

Inside a transaction, use `tx.Collection[User]("archived_users")`.
When the schema is intentionally dynamic, use `db.Collection[mongo.M](ctx, name)`.

## Index tags

```go
type Job struct {
	ID     string `bson:"_id"`
	TaskID string `bson:"task_id" db:"index,unique=job_task_url"`
	URL    string `bson:"url" db:"unique=job_task_url"`
	Status string `bson:"status" db:"index"`
}

err := db.EnsureIndexes[Job](ctx)
```

Supported tags are `pk`, `index`, `unique`, `index=group`, and `unique=group`.

## Errors

The common errors support `errors.Is`:

- `ErrRecordNotFound`
- `ErrDuplicateKey`
- `ErrNoID`
- `ErrInvalidModelName`
- `ErrEmptyFilter`

## Migration from v0.3

This release intentionally has no compatibility layer.

| v0.3 | Go 1.27 API |
| --- | --- |
| `DB.Txn(ctx, fn)` used only for context | call `DB.Save/Get/Find/...` with `ctx` directly |
| `txn.Model(model).Set(model)` | `db.Save(ctx, model)` |
| `txn.Model(&User{}).Unmarshal(id, user)` | `user, err := db.Get[User](ctx, id)` |
| `List` plus `ToEntity[User]` | `db.Find[User](ctx, filter).Each(...)` |
| `Pagination` returning `[]M` | `db.Find[User](ctx, filter).Page(...)` |
| `txn.Model("archive")` | `db.Collection[User](ctx, "archive")` |
| `DB.Txn(ctx, fn, true)` | `DB.Transaction(ctx, fn)` |

## Tests

```bash
go test -race -cover ./...
```

The default suite uses the MongoDB driver's mock deployment, so CRUD, query,
pagination, index, and transaction-scope tests do not require a running server.
CI enforces at least 85% statement coverage. The `integration` suite below
verifies the same API against a real replica set.

Integration tests use the `integration` build tag and a MongoDB replica set:

```bash
docker run --rm -d --name mongo-sdk-integration \
  -p 127.0.0.1:37027:27017 \
  mongo:8.2 --replSet rs0 --bind_ip_all --port 27017

docker exec mongo-sdk-integration mongosh --quiet --eval \
  'rs.initiate({_id:"rs0",members:[{_id:0,host:"localhost:27017"}]})'

MONGODB_URI='mongodb://127.0.0.1:37027/?replicaSet=rs0&directConnection=true' \
  go test -race -tags=integration ./...

docker stop mongo-sdk-integration
```
