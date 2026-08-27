//go:build integration

package mongo_test

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	sdkmongo "github.com/liran/mongo"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
	driver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"
)

type integrationProfile struct {
	City string `bson:"city" db:"index=profile_city"`
}

type integrationUser struct {
	ID      string             `bson:"_id"`
	Name    string             `bson:"name,omitempty"`
	Email   string             `bson:"email,omitempty" db:"unique"`
	Age     int64              `bson:"age,omitempty" db:"index"`
	Score   int64              `bson:"score,omitempty"`
	Active  bool               `bson:"active,omitempty" db:"index"`
	Profile integrationProfile `bson:"profile"`
}

type integrationAudit struct {
	ID        string `bson:"_id"`
	Message   string `bson:"message" db:"index=audit_message"`
	CreatedAt int64  `db:"index=audit_created"`
}

func (*integrationAudit) CollectionName() string {
	return "integration_audits"
}

func TestTypedSDK(t *testing.T) {
	db, ctx := openIntegrationDatabase(t)

	err := db.EnsureIndexes[integrationUser](ctx)
	require.NoError(t, err)
	err = db.EnsureIndexes[integrationAudit](ctx)
	require.NoError(t, err)

	userIndex := integrationIndexExpectation{
		Collection: sdkmongo.CollectionName[integrationUser](),
		Name:       "profile_city",
		Keys:       bson.D{{Key: "profile.city", Value: int32(1)}},
	}
	requireIntegrationIndex(t, ctx, db, userIndex)
	auditMessageIndex := integrationIndexExpectation{
		Collection: sdkmongo.CollectionName[integrationAudit](),
		Name:       "audit_message",
		Keys:       bson.D{{Key: "message", Value: int32(1)}},
	}
	requireIntegrationIndex(t, ctx, db, auditMessageIndex)
	auditCreatedIndex := integrationIndexExpectation{
		Collection: sdkmongo.CollectionName[integrationAudit](),
		Name:       "audit_created",
		Keys:       bson.D{{Key: "createdat", Value: int32(1)}},
	}
	requireIntegrationIndex(t, ctx, db, auditCreatedIndex)

	users := []*integrationUser{
		{ID: "001", Name: "Alice", Email: "alice@example.com", Age: 31, Score: 10, Active: true},
		{ID: "002", Name: "Bob", Email: "bob@example.com", Age: 25, Score: 20, Active: true},
		{ID: "003", Name: "Carol", Email: "carol@example.com", Age: 40, Score: 30, Active: false},
	}
	err = db.SaveMany(ctx, users...)
	require.NoError(t, err)

	found, err := db.Get[integrationUser](ctx, "001")
	require.NoError(t, err)
	require.Equal(t, users[0], found)

	destination, err := db.Get[integrationUser](ctx, "002")
	require.NoError(t, err)
	require.Equal(t, users[1], destination)

	_, err = db.Get[integrationUser](ctx, "missing")
	require.ErrorIs(t, err, sdkmongo.ErrRecordNotFound)

	duplicate := &integrationUser{ID: "004", Email: "alice@example.com"}
	err = db.Save(ctx, duplicate)
	require.ErrorIs(t, err, sdkmongo.ErrDuplicateKey)

	ageUpdate := sdkmongo.M{"age": int64(32)}
	updated, err := db.Update[integrationUser](ctx, "001", ageUpdate)
	require.NoError(t, err)
	require.Equal(t, int64(32), updated.Age)
	require.Equal(t, "Alice", updated.Name)

	fields := sdkmongo.NewDocument().Set("name", "Alice Updated")
	updated, err = db.Update[integrationUser](ctx, "001", fields)
	require.NoError(t, err)
	require.Equal(t, "Alice Updated", updated.Name)

	updated, err = db.Increment[integrationUser](ctx, "001", sdkmongo.NewDocument().Set("score", 5))
	require.NoError(t, err)
	require.Equal(t, int64(15), updated.Score)

	exists, err := db.Find[integrationUser](ctx, sdkmongo.NewDocument().Set("email", "alice@example.com")).Exists()
	require.NoError(t, err)
	require.True(t, exists)

	activeFilter := sdkmongo.NewDocument().Set("active", true)
	count, err := db.Find[integrationUser](ctx, activeFilter).Count()
	require.NoError(t, err)
	require.Equal(t, int64(2), count)
	baseQuery := db.Find[integrationUser](ctx, activeFilter)
	limited, err := baseQuery.Limit(1).All()
	require.NoError(t, err)
	require.Len(t, limited, 1)
	allActive, err := baseQuery.All()
	require.NoError(t, err)
	require.Len(t, allActive, 2)

	page, err := db.Find[integrationUser](ctx, activeFilter).
		Sort(sdkmongo.NewDocument().Set("_id", 1)).
		Page(1, 1)
	require.NoError(t, err)
	require.Equal(t, int64(2), page.Total)
	require.Equal(t, int64(1), page.Number)
	require.Len(t, page.Items, 1)
	require.Equal(t, "001", page.Items[0].ID)

	first, err := db.Find[integrationUser](ctx).Sort(sdkmongo.NewDocument().Set("age", -1)).First()
	require.NoError(t, err)
	require.Equal(t, "003", first.ID)

	next, err := db.Find[integrationUser](ctx).AfterID("001").Limit(2).All()
	require.NoError(t, err)
	require.Len(t, next, 2)
	require.Equal(t, "002", next[0].ID)
	require.Equal(t, "003", next[1].ID)
	remaining, err := db.Find[integrationUser](ctx).AfterID("001").Count()
	require.NoError(t, err)
	require.Equal(t, int64(2), remaining)

	previous, err := db.Find[integrationUser](ctx).BeforeID("003").Limit(2).All()
	require.NoError(t, err)
	require.Len(t, previous, 2)
	require.Equal(t, "002", previous[0].ID)
	require.Equal(t, "001", previous[1].ID)

	listedIDs := make([]string, 0, len(users))
	err = db.Find[integrationUser](ctx).BatchSize(2).Each(func(user *integrationUser) (bool, error) {
		listedIDs = append(listedIDs, user.ID)
		return true, nil
	})
	require.NoError(t, err)
	require.Equal(t, []string{"001", "002", "003"}, listedIDs)

	updateFilter := sdkmongo.NewDocument().Set("active", true)
	updateFields := sdkmongo.NewDocument().Set("active", false)
	_, err = db.UpdateMany[integrationUser](ctx, nil, updateFields)
	require.ErrorIs(t, err, sdkmongo.ErrEmptyFilter)
	updatedCount, err := db.UpdateMany[integrationUser](ctx, updateFilter, updateFields)
	require.NoError(t, err)
	require.Equal(t, int64(2), updatedCount)

	err = db.Transaction(ctx, func(tx *sdkmongo.Tx) error {
		audit := &integrationAudit{ID: "audit-001", Message: "created"}
		if err := tx.Save(audit); err != nil {
			return err
		}

		archived := &integrationUser{ID: "archived-001", Name: "Archived"}
		return tx.Collection[integrationUser]("archived_users").Save(archived)
	})
	require.NoError(t, err)

	audit, err := db.Get[integrationAudit](ctx, "audit-001")
	require.NoError(t, err)
	require.Equal(t, "created", audit.Message)

	archived, err := db.Collection[integrationUser](ctx, "archived_users").Get("archived-001")
	require.NoError(t, err)
	require.Equal(t, "Archived", archived.Name)

	dynamic := db.Collection[sdkmongo.M](ctx, "runtime_documents")
	raw := sdkmongo.M{"_id": "raw-001", "kind": "job"}
	err = dynamic.Save(&raw)
	require.NoError(t, err)
	loadedRaw, err := dynamic.Get("raw-001")
	require.NoError(t, err)
	require.Equal(t, "job", (*loadedRaw)["kind"])

	err = db.Delete[integrationUser](ctx, "003")
	require.NoError(t, err)
	has, err := db.Find[integrationUser](ctx, sdkmongo.M{"_id": "003"}).Exists()
	require.NoError(t, err)
	require.False(t, has)

	deleted, err := db.DeleteMany[integrationUser](ctx, sdkmongo.NewDocument().Set("active", false))
	require.NoError(t, err)
	require.Equal(t, int64(2), deleted)

	canceledCtx, cancel := context.WithCancel(ctx)
	cancel()
	_, err = db.Get[integrationUser](canceledCtx, "001")
	require.ErrorIs(t, err, context.Canceled)
}

type integrationIndexExpectation struct {
	Collection string
	Name       string
	Keys       bson.D
	Unique     bool
}

func requireIntegrationIndex(
	t *testing.T,
	ctx context.Context,
	db *sdkmongo.Database,
	expected integrationIndexExpectation,
) {
	t.Helper()
	specifications, err := db.Database.Collection(expected.Collection).Indexes().ListSpecifications(ctx)
	require.NoError(t, err)
	for _, specification := range specifications {
		if specification.Name != expected.Name {
			continue
		}
		keys := bson.D{}
		err = bson.Unmarshal(specification.KeysDocument, &keys)
		require.NoError(t, err)
		require.Equal(t, expected.Keys, keys)
		unique := specification.Unique != nil && *specification.Unique
		require.Equal(t, expected.Unique, unique)
		return
	}
	t.Fatalf("index %q was not created", expected.Name)
}

func openIntegrationDatabase(t *testing.T) (*sdkmongo.Database, context.Context) {
	t.Helper()

	uri := os.Getenv("MONGODB_URI")
	if uri == "" {
		uri = "mongodb://127.0.0.1:27017/?replicaSet=rs0&directConnection=true"
	}

	db, err := sdkmongo.Open(uri, "mongo_sdk_integration")
	require.NoError(t, err)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)

	err = db.Ping(ctx, readpref.Primary())
	require.NoError(t, err)
	err = db.Database.Drop(ctx)
	require.NoError(t, err)
	t.Cleanup(func() {
		dropCtx, dropCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer dropCancel()
		_ = db.Database.Drop(dropCtx)
		_ = db.Close()
	})

	return db, ctx
}

func TestTypedSDKTransactionRollback(t *testing.T) {
	db, ctx := openIntegrationDatabase(t)

	wantErr := errors.New("rollback")
	err := db.Transaction(ctx, func(tx *sdkmongo.Tx) error {
		audit := &integrationAudit{ID: "rollback-001", Message: "rollback"}
		if err := tx.Save(audit); err != nil {
			return err
		}
		return wantErr
	})
	require.ErrorIs(t, err, wantErr)

	_, err = db.Get[integrationAudit](ctx, "rollback-001")
	require.ErrorIs(t, err, sdkmongo.ErrRecordNotFound)
}

func TestEnsureIndexesRejectsExistingOptionConflict(t *testing.T) {
	db, ctx := openIntegrationDatabase(t)
	collectionName := sdkmongo.CollectionName[integrationUser]()
	emailKeys := bson.D{{Key: "email", Value: 1}}
	indexModel := driver.IndexModel{Keys: emailKeys}
	_, err := db.Database.Collection(collectionName).Indexes().CreateOne(ctx, indexModel)
	require.NoError(t, err)

	err = db.EnsureIndexes[integrationUser](ctx)
	require.ErrorIs(t, err, sdkmongo.ErrIndexConflict)
}
