package mongo_test

import (
	"context"
	"log"
	"os"
	"testing"

	"github.com/joho/godotenv"
	"github.com/liran/mongo"
	"github.com/stretchr/testify/require"
)

func TestPipeline(t *testing.T) {
	err := godotenv.Load()
	require.NoError(t, err)

	uri := os.Getenv("MONGO_URI")
	require.NotEmpty(t, uri)

	ctx := context.Background()

	db := mongo.NewDatabase(uri, "test", func(c mongo.ClientOptions) {
		ttt := false
		c.Direct = &ttt
	})

	db.Database.Drop(ctx)

	type User struct {
		ID         string `bson:"_id"`
		Name       string
		Age        int64
		OrderCount string `bson:"order_count,omitempty"`
	}

	type Book struct {
		ID   string `bson:"_id"`
		Name string
	}

	// init collection
	user := &User{ID: "2", Name: "Name2", Age: 2, OrderCount: "1234"}
	book := &Book{ID: "1", Name: "Book1"}
	err = db.Txn(ctx, func(txn *mongo.Txn) error {
		err := txn.Model(user).Set(user)
		if err != nil {
			return err
		}
		return txn.Model(book).Set(book)
	}, false)
	require.NoError(t, err)

	// get not found
	err = db.Txn(ctx, func(txn *mongo.Txn) error {
		a := &User{}
		return txn.Model(a).Unmarshal("1", a)
	}, true)
	require.ErrorIs(t, err, mongo.ErrRecordNotFound)

	user1 := &User{}
	err = db.Unmarshal(user.ID, user1)
	require.NoError(t, err)
	require.Equal(t, user, user1)

	// transaction
	err = db.Txn(ctx, func(txn *mongo.Txn) error {
		_, err := txn.Model(user).Get(user.ID)
		if err != nil {
			return err
		}

		return txn.Model(book).Set(book)
	}, true)
	require.NoError(t, err)

	user1 = &User{}
	err = db.Unmarshal(user.ID, user1)
	require.NoError(t, err)
	require.Equal(t, user, user1)

	// pagination
	err = db.Txn(ctx, func(txn *mongo.Txn) error {
		total, list, err := txn.Model("user").Pagination(nil, nil, 1, 10)
		require.NoError(t, err)
		require.Equal(t, int64(1), total)

		users := mongo.ToEntities[User](list)
		for _, user := range users {
			require.Equal(t, user1, user)
		}
		return nil
	})
	require.NoError(t, err)

	// list
	err = db.Txn(ctx, func(txn *mongo.Txn) error {
		return txn.Model(user).List(nil, 1, func(m mongo.M, total int64) (bool, error) {
			user := mongo.ToEntity[User](m)
			log.Printf("total: %d, id: %s, name: %s", total, user.ID, user.Name)
			return true, nil
		})
	})
	require.NoError(t, err)

	// count
	err = db.Txn(ctx, func(txn *mongo.Txn) error {
		count, err := txn.Model("user").Count(nil)
		if err != nil {
			return err
		}
		log.Println("count:", count)
		return nil
	})
	require.NoError(t, err)

	// inc
	err = db.Txn(ctx, func(txn *mongo.Txn) error {
		return txn.Model("user").Inc("2", mongo.Map().Set("lala", -1).Set("inc", 1))
	}, true)
	require.NoError(t, err)

	// del
	err = db.Txn(ctx, func(txn *mongo.Txn) error {
		return txn.Model("user").Del("1")
	})
	require.NoError(t, err)

	// first
	err = db.Txn(ctx, func(txn *mongo.Txn) error {
		res, err := txn.Model("user").First(nil, mongo.Map().Set("age", -1))
		if err != nil {
			return err
		}

		user := mongo.ToEntity[User](res)
		log.Printf("%+v", user)

		return nil
	})
	require.NoError(t, err)
}
