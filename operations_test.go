package mongo

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	driver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/drivertest"
	"go.mongodb.org/mongo-driver/v2/x/mongo/driver/xoptions"
)

type operationUser struct {
	ID     string `bson:"_id"`
	Name   string `bson:"name,omitempty"`
	Active bool   `bson:"active"`
	Score  int64  `bson:"score,omitempty"`
}

type indexedOperationUser struct {
	ID    string `bson:"_id"`
	Email string `bson:"email" db:"unique"`
}

type regularIndexedOperationUser struct {
	ID    string `bson:"_id"`
	Email string `bson:"email" db:"index"`
}

type namedIndexedOperationUser struct {
	ID    string `bson:"_id"`
	Email string `bson:"email" db:"unique=user_email"`
}

type namedOperationUser struct {
	ID string `bson:"_id"`
}

func (*namedOperationUser) CollectionName() string {
	return "named_users"
}

func newMockDatabase(t *testing.T, responses ...bson.D) *Database {
	t.Helper()

	deployment := drivertest.NewMockDeployment(responses...)
	clientOptions := options.Client()
	err := xoptions.SetInternalClientOptions(clientOptions, "deployment", deployment)
	require.NoError(t, err)
	client, err := driver.Connect(clientOptions)
	require.NoError(t, err)

	wrapper := &Client{Client: client}
	database := &Database{Client: wrapper, Database: client.Database("unit")}
	t.Cleanup(func() {
		require.NoError(t, database.Close())
	})
	return database
}

func successResponse(elements ...bson.E) bson.D {
	response := bson.D{{Key: "ok", Value: 1}}
	response = append(response, elements...)
	return response
}

func writeResponse(matched, modified int64) bson.D {
	matchedElement := bson.E{Key: "n", Value: matched}
	modifiedElement := bson.E{Key: "nModified", Value: modified}
	return successResponse(matchedElement, modifiedElement)
}

func deleteResponse(deleted int64) bson.D {
	deletedElement := bson.E{Key: "n", Value: deleted}
	return successResponse(deletedElement)
}

func cursorResponse(namespace string, documents ...bson.D) bson.D {
	batch := make(bson.A, 0, len(documents))
	for _, document := range documents {
		batch = append(batch, document)
	}
	cursor := bson.D{
		{Key: "id", Value: int64(0)},
		{Key: "ns", Value: namespace},
		{Key: "firstBatch", Value: batch},
	}
	cursorElement := bson.E{Key: "cursor", Value: cursor}
	return successResponse(cursorElement)
}

func findAndModifyResponse(document bson.D) bson.D {
	valueElement := bson.E{Key: "value", Value: document}
	return successResponse(valueElement)
}

func duplicateKeyResponse() bson.D {
	writeError := bson.D{
		{Key: "index", Value: 0},
		{Key: "code", Value: 11000},
		{Key: "errmsg", Value: "duplicate key"},
	}
	writeErrors := bson.A{writeError}
	response := bson.D{
		{Key: "ok", Value: 1},
		{Key: "writeErrors", Value: writeErrors},
	}
	return response
}

func TestDatabaseWriteOperations(t *testing.T) {
	ctx := context.Background()

	t.Run("save", func(t *testing.T) {
		response := writeResponse(1, 1)
		db := newMockDatabase(t, response)
		user := &operationUser{ID: "user-1", Name: "Liran"}

		err := db.Save(ctx, user)
		require.NoError(t, err)
	})

	t.Run("save validates id before writing", func(t *testing.T) {
		db := newMockDatabase(t)
		user := new(operationUser)

		err := db.Save(ctx, user)
		require.ErrorIs(t, err, ErrNoID)
	})

	t.Run("save normalizes duplicate key", func(t *testing.T) {
		response := duplicateKeyResponse()
		db := newMockDatabase(t, response)
		user := &operationUser{ID: "user-1"}

		err := db.Save(ctx, user)
		require.ErrorIs(t, err, ErrDuplicateKey)
	})

	t.Run("save many", func(t *testing.T) {
		response := writeResponse(2, 2)
		db := newMockDatabase(t, response)
		first := &operationUser{ID: "user-1"}
		second := &operationUser{ID: "user-2"}

		err := db.SaveMany(ctx, first, second)
		require.NoError(t, err)
	})

	t.Run("save many handles empty input", func(t *testing.T) {
		db := newMockDatabase(t)
		err := db.SaveMany[operationUser](ctx)
		require.NoError(t, err)
	})

	t.Run("save many validates every id", func(t *testing.T) {
		db := newMockDatabase(t)
		first := &operationUser{ID: "user-1"}
		second := new(operationUser)

		err := db.SaveMany(ctx, first, second)
		require.ErrorIs(t, err, ErrNoID)
	})

	t.Run("update", func(t *testing.T) {
		document := bson.D{
			{Key: "_id", Value: "user-1"},
			{Key: "name", Value: "Updated"},
			{Key: "active", Value: false},
		}
		response := findAndModifyResponse(document)
		db := newMockDatabase(t, response)
		fields := M{"name": "Updated", "active": false}

		user, err := db.Update[operationUser](ctx, "user-1", fields)
		require.NoError(t, err)
		require.Equal(t, "Updated", user.Name)
		require.False(t, user.Active)
	})

	t.Run("update rejects unencodable fields", func(t *testing.T) {
		db := newMockDatabase(t)
		fields := make(chan int)

		_, err := db.Update[operationUser](ctx, "user-1", fields)
		require.Error(t, err)
	})

	t.Run("increment", func(t *testing.T) {
		document := bson.D{
			{Key: "_id", Value: "user-1"},
			{Key: "score", Value: int64(11)},
		}
		response := findAndModifyResponse(document)
		db := newMockDatabase(t, response)
		fields := M{"score": 1}

		user, err := db.Increment[operationUser](ctx, "user-1", fields)
		require.NoError(t, err)
		require.Equal(t, int64(11), user.Score)
	})

	t.Run("delete", func(t *testing.T) {
		response := deleteResponse(1)
		db := newMockDatabase(t, response)

		err := db.Delete[operationUser](ctx, "user-1")
		require.NoError(t, err)
	})

	t.Run("update many", func(t *testing.T) {
		response := writeResponse(3, 2)
		db := newMockDatabase(t, response)
		filter := M{"active": true}
		fields := M{"active": false}

		modified, err := db.UpdateMany[operationUser](ctx, filter, fields)
		require.NoError(t, err)
		require.Equal(t, int64(2), modified)
	})

	t.Run("update many rejects empty filter", func(t *testing.T) {
		db := newMockDatabase(t)
		fields := M{"active": false}

		_, err := db.UpdateMany[operationUser](ctx, nil, fields)
		require.ErrorIs(t, err, ErrEmptyFilter)
	})

	t.Run("delete many", func(t *testing.T) {
		response := deleteResponse(3)
		db := newMockDatabase(t, response)
		filter := M{"active": false}

		deleted, err := db.DeleteMany[operationUser](ctx, filter)
		require.NoError(t, err)
		require.Equal(t, int64(3), deleted)
	})

	t.Run("delete many rejects empty filter", func(t *testing.T) {
		db := newMockDatabase(t)

		_, err := db.DeleteMany[operationUser](ctx, M{})
		require.ErrorIs(t, err, ErrEmptyFilter)
	})
}

func TestDatabaseReadOperations(t *testing.T) {
	ctx := context.Background()
	firstDocument := bson.D{
		{Key: "_id", Value: "user-1"},
		{Key: "name", Value: "Alice"},
		{Key: "active", Value: true},
	}
	secondDocument := bson.D{
		{Key: "_id", Value: "user-2"},
		{Key: "name", Value: "Bob"},
		{Key: "active", Value: true},
	}

	t.Run("get", func(t *testing.T) {
		response := cursorResponse("unit.operation_user", firstDocument)
		db := newMockDatabase(t, response)

		user, err := db.Get[operationUser](ctx, "user-1")
		require.NoError(t, err)
		require.Equal(t, "Alice", user.Name)
	})

	t.Run("get missing", func(t *testing.T) {
		response := cursorResponse("unit.operation_user")
		db := newMockDatabase(t, response)

		_, err := db.Get[operationUser](ctx, "missing")
		require.ErrorIs(t, err, ErrRecordNotFound)
	})

	t.Run("find all", func(t *testing.T) {
		response := cursorResponse("unit.operation_user", firstDocument, secondDocument)
		db := newMockDatabase(t, response)
		filter := M{"active": true}
		sort := M{"_id": 1}
		projection := M{"name": 1}

		users, err := db.Find[operationUser](ctx, filter).
			Sort(sort).
			Select(projection).
			Limit(2).
			All()
		require.NoError(t, err)
		require.Len(t, users, 2)
		require.Equal(t, "Alice", users[0].Name)
	})

	t.Run("find first", func(t *testing.T) {
		response := cursorResponse("unit.operation_user", secondDocument)
		db := newMockDatabase(t, response)
		sort := M{"name": -1}

		user, err := db.Find[operationUser](ctx).Sort(sort).First()
		require.NoError(t, err)
		require.Equal(t, "Bob", user.Name)
	})

	t.Run("find first with ID cursor", func(t *testing.T) {
		response := cursorResponse("unit.operation_user", secondDocument)
		db := newMockDatabase(t, response)
		projection := M{"name": 1}

		user, err := db.Find[operationUser](ctx).AfterID("user-1").Select(projection).First()
		require.NoError(t, err)
		require.Equal(t, "Bob", user.Name)
	})

	t.Run("descending cursor", func(t *testing.T) {
		response := cursorResponse("unit.operation_user", firstDocument)
		db := newMockDatabase(t, response)

		users, err := db.Find[operationUser](ctx).BeforeID("user-2").All()
		require.NoError(t, err)
		require.Len(t, users, 1)
	})

	t.Run("ascending cursor", func(t *testing.T) {
		response := cursorResponse("unit.operation_user", secondDocument)
		db := newMockDatabase(t, response)

		users, err := db.Find[operationUser](ctx).AfterID("user-1").All()
		require.NoError(t, err)
		require.Len(t, users, 1)
	})

	t.Run("count", func(t *testing.T) {
		countDocument := bson.D{{Key: "n", Value: int64(2)}}
		response := cursorResponse("unit.operation_user", countDocument)
		db := newMockDatabase(t, response)

		count, err := db.Find[operationUser](ctx, M{"active": true}).Count()
		require.NoError(t, err)
		require.Equal(t, int64(2), count)
	})

	t.Run("unfiltered count uses estimate", func(t *testing.T) {
		response := successResponse(bson.E{Key: "n", Value: int64(2)})
		db := newMockDatabase(t, response)

		count, err := db.Find[operationUser](ctx).Count()
		require.NoError(t, err)
		require.Equal(t, int64(2), count)
	})

	t.Run("exists", func(t *testing.T) {
		countDocument := bson.D{{Key: "n", Value: int64(1)}}
		response := cursorResponse("unit.operation_user", countDocument)
		db := newMockDatabase(t, response)

		exists, err := db.Find[operationUser](ctx, M{"name": "Alice"}).Exists()
		require.NoError(t, err)
		require.True(t, exists)
	})

	t.Run("unfiltered exists uses estimate", func(t *testing.T) {
		response := successResponse(bson.E{Key: "n", Value: int64(1)})
		db := newMockDatabase(t, response)

		exists, err := db.Find[operationUser](ctx).Exists()
		require.NoError(t, err)
		require.True(t, exists)
	})

	t.Run("page", func(t *testing.T) {
		countDocument := bson.D{{Key: "n", Value: int64(2)}}
		countResponse := cursorResponse("unit.operation_user", countDocument)
		findResponse := cursorResponse("unit.operation_user", firstDocument)
		db := newMockDatabase(t, countResponse, findResponse)

		page, err := db.Find[operationUser](ctx, M{"active": true}).Page(1, 1)
		require.NoError(t, err)
		require.Equal(t, int64(2), page.Total)
		require.Len(t, page.Items, 1)
	})

	t.Run("unfiltered empty page uses estimate and defaults", func(t *testing.T) {
		response := successResponse(bson.E{Key: "n", Value: int64(0)})
		db := newMockDatabase(t, response)

		page, err := db.Find[operationUser](ctx).Page(0, 0)
		require.NoError(t, err)
		require.Equal(t, int64(1), page.Number)
		require.Equal(t, int64(20), page.PageSize)
		require.Empty(t, page.Items)
		require.NotNil(t, page.Items)
	})

	t.Run("each", func(t *testing.T) {
		firstBatch := cursorResponse("unit.operation_user", firstDocument, secondDocument)
		thirdDocument := bson.D{{Key: "_id", Value: "user-3"}, {Key: "name", Value: "Carol"}}
		secondBatch := cursorResponse("unit.operation_user", thirdDocument)
		db := newMockDatabase(t, firstBatch, secondBatch)
		ids := make([]string, 0, 3)

		err := db.Find[operationUser](ctx).BatchSize(2).Each(func(user *operationUser) (bool, error) {
			ids = append(ids, user.ID)
			return true, nil
		})
		require.NoError(t, err)
		require.Equal(t, []string{"user-1", "user-2", "user-3"}, ids)
	})

	t.Run("each stops successfully", func(t *testing.T) {
		response := cursorResponse("unit.operation_user", firstDocument, secondDocument)
		db := newMockDatabase(t, response)
		visited := 0

		err := db.Find[operationUser](ctx).Each(func(user *operationUser) (bool, error) {
			visited++
			return false, nil
		})
		require.NoError(t, err)
		require.Equal(t, 1, visited)
	})

	t.Run("each returns callback error", func(t *testing.T) {
		response := cursorResponse("unit.operation_user", firstDocument)
		db := newMockDatabase(t, response)
		wantErr := errors.New("stop")

		err := db.Find[operationUser](ctx).Each(func(user *operationUser) (bool, error) {
			return false, wantErr
		})
		require.ErrorIs(t, err, wantErr)
	})
}

func TestCollectionAndIndexes(t *testing.T) {
	ctx := context.Background()

	t.Run("explicit collection", func(t *testing.T) {
		response := writeResponse(1, 1)
		db := newMockDatabase(t, response)
		collection := db.Collection[operationUser](ctx, "custom_users")
		require.NotNil(t, collection.Raw())
		user := &operationUser{ID: "user-1"}
		require.NoError(t, collection.Save(user))
	})

	t.Run("empty collection name panics", func(t *testing.T) {
		db := newMockDatabase(t)
		require.PanicsWithValue(t, ErrInvalidModelName, func() {
			db.Collection[operationUser](ctx, "")
		})
	})

	t.Run("custom collection name", func(t *testing.T) {
		response := writeResponse(1, 1)
		db := newMockDatabase(t, response)
		user := &namedOperationUser{ID: "user-1"}
		require.NoError(t, db.Save(ctx, user))
	})

	t.Run("no declared indexes", func(t *testing.T) {
		db := newMockDatabase(t)
		require.NoError(t, db.EnsureIndexes[operationUser](ctx))
	})

	t.Run("invalid index model", func(t *testing.T) {
		db := newMockDatabase(t)
		require.ErrorIs(t, db.EnsureIndexes[map[string]any](ctx), ErrInvalidModelName)
	})

	t.Run("creates missing index", func(t *testing.T) {
		idKeys := bson.D{{Key: "_id", Value: 1}}
		idSpecification := bson.D{
			{Key: "v", Value: int32(2)},
			{Key: "key", Value: idKeys},
			{Key: "name", Value: "_id_"},
		}
		listResponse := cursorResponse("unit.indexed_operation_user", idSpecification)
		createResponse := successResponse()
		db := newMockDatabase(t, listResponse, createResponse)

		err := db.EnsureIndexes[indexedOperationUser](ctx)
		require.NoError(t, err)
	})

	t.Run("keeps existing index", func(t *testing.T) {
		emailKeys := bson.D{{Key: "email", Value: 1}}
		emailSpecification := bson.D{
			{Key: "v", Value: int32(2)},
			{Key: "key", Value: emailKeys},
			{Key: "name", Value: "email_1"},
			{Key: "unique", Value: true},
		}
		listResponse := cursorResponse("unit.indexed_operation_user", emailSpecification)
		db := newMockDatabase(t, listResponse)

		err := db.EnsureIndexes[indexedOperationUser](ctx)
		require.NoError(t, err)
	})

	t.Run("rejects weaker existing index", func(t *testing.T) {
		emailKeys := bson.D{{Key: "email", Value: 1}}
		emailSpecification := bson.D{
			{Key: "v", Value: int32(2)},
			{Key: "key", Value: emailKeys},
			{Key: "name", Value: "email_1"},
		}
		listResponse := cursorResponse("unit.indexed_operation_user", emailSpecification)
		db := newMockDatabase(t, listResponse)

		err := db.EnsureIndexes[indexedOperationUser](ctx)
		require.ErrorIs(t, err, ErrIndexConflict)
	})

	t.Run("accepts stronger existing index", func(t *testing.T) {
		emailKeys := bson.D{{Key: "email", Value: 1}}
		emailSpecification := bson.D{
			{Key: "v", Value: int32(2)},
			{Key: "key", Value: emailKeys},
			{Key: "name", Value: "email_1"},
			{Key: "unique", Value: true},
		}
		listResponse := cursorResponse("unit.regular_indexed_operation_user", emailSpecification)
		db := newMockDatabase(t, listResponse)

		err := db.EnsureIndexes[regularIndexedOperationUser](ctx)
		require.NoError(t, err)
	})

	t.Run("rejects custom name mismatch", func(t *testing.T) {
		emailKeys := bson.D{{Key: "email", Value: 1}}
		emailSpecification := bson.D{
			{Key: "v", Value: int32(2)},
			{Key: "key", Value: emailKeys},
			{Key: "name", Value: "legacy_email"},
			{Key: "unique", Value: true},
		}
		listResponse := cursorResponse("unit.named_indexed_operation_user", emailSpecification)
		db := newMockDatabase(t, listResponse)

		err := db.EnsureIndexes[namedIndexedOperationUser](ctx)
		require.ErrorIs(t, err, ErrIndexConflict)
	})

	t.Run("rejects custom name key mismatch", func(t *testing.T) {
		usernameKeys := bson.D{{Key: "username", Value: 1}}
		usernameSpecification := bson.D{
			{Key: "v", Value: int32(2)},
			{Key: "key", Value: usernameKeys},
			{Key: "name", Value: "user_email"},
			{Key: "unique", Value: true},
		}
		listResponse := cursorResponse("unit.named_indexed_operation_user", usernameSpecification)
		db := newMockDatabase(t, listResponse)

		err := db.EnsureIndexes[namedIndexedOperationUser](ctx)
		require.ErrorIs(t, err, ErrIndexConflict)
	})

	t.Run("accepts matching custom name", func(t *testing.T) {
		emailKeys := bson.D{{Key: "email", Value: 1}}
		emailSpecification := bson.D{
			{Key: "v", Value: int32(2)},
			{Key: "key", Value: emailKeys},
			{Key: "name", Value: "user_email"},
			{Key: "unique", Value: true},
		}
		listResponse := cursorResponse("unit.named_indexed_operation_user", emailSpecification)
		db := newMockDatabase(t, listResponse)

		err := db.EnsureIndexes[namedIndexedOperationUser](ctx)
		require.NoError(t, err)
	})

	t.Run("returns create index error", func(t *testing.T) {
		idKeys := bson.D{{Key: "_id", Value: 1}}
		idSpecification := bson.D{
			{Key: "v", Value: int32(2)},
			{Key: "key", Value: idKeys},
			{Key: "name", Value: "_id_"},
		}
		listResponse := cursorResponse("unit.indexed_operation_user", idSpecification)
		createError := bson.D{
			{Key: "ok", Value: 0},
			{Key: "code", Value: int32(85)},
			{Key: "errmsg", Value: "index conflict"},
		}
		db := newMockDatabase(t, listResponse, createError)

		err := db.EnsureIndexes[indexedOperationUser](ctx)
		require.Error(t, err)
	})
}

func TestTransactionOperationScope(t *testing.T) {
	ctx := context.Background()
	getDocument := bson.D{{Key: "_id", Value: "user-1"}}
	updateDocument := bson.D{{Key: "_id", Value: "user-1"}, {Key: "name", Value: "updated"}}
	incrementDocument := bson.D{{Key: "_id", Value: "user-1"}, {Key: "score", Value: int64(2)}}
	findDocument := bson.D{{Key: "_id", Value: "user-1"}}
	responses := []bson.D{
		writeResponse(1, 1),
		writeResponse(2, 2),
		cursorResponse("unit.operation_user", getDocument),
		findAndModifyResponse(updateDocument),
		findAndModifyResponse(incrementDocument),
		deleteResponse(1),
		writeResponse(2, 1),
		deleteResponse(2),
		cursorResponse("unit.operation_user", findDocument),
	}
	db := newMockDatabase(t, responses...)
	tx := &Tx{ctx: ctx, database: db}
	first := &operationUser{ID: "user-1"}
	second := &operationUser{ID: "user-2"}
	require.NoError(t, tx.Save(first))
	require.NoError(t, tx.SaveMany(first, second))
	_, err := tx.Get[operationUser]("user-1")
	require.NoError(t, err)
	_, err = tx.Update[operationUser]("user-1", M{"name": "updated"})
	require.NoError(t, err)
	_, err = tx.Increment[operationUser]("user-1", M{"score": 1})
	require.NoError(t, err)
	require.NoError(t, tx.Delete[operationUser]("user-1"))
	_, err = tx.UpdateMany[operationUser](M{"active": true}, M{"active": false})
	require.NoError(t, err)
	_, err = tx.DeleteMany[operationUser](M{"active": false})
	require.NoError(t, err)
	_, err = tx.Find[operationUser]().All()
	require.NoError(t, err)
	require.NotNil(t, tx.Collection[operationUser]("custom_users").Raw())
}

func TestTransactionReturnsCallbackError(t *testing.T) {
	db := newMockDatabase(t)
	wantErr := errors.New("abort")
	err := db.Transaction(context.Background(), func(tx *Tx) error {
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)
}
