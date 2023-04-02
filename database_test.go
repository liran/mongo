package mongo

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/pkg/errors"
)

func TestCRUD(t *testing.T) {
	db := NewDatabase("mongodb://172.31.10.100:27017/?directConnection=true", "test")
	defer db.Close()

	type User struct {
		ID         string `db:"pk" bson:"_id"`
		Name       string
		Age        int64
		OrderCount string `bson:"order_count"`
	}
	ctx := context.Background()

	// get not found
	err := db.Txn(ctx, func(txn *Txn) error {
		a := &User{}
		return txn.Model(a).Unmarshal("1", a)
	}, false)
	if err != nil && !errors.Is(err, ErrNotFoundModel) {
		t.Fatal(err)
	}

	// set
	err = db.Txn(ctx, func(txn *Txn) error {
		user := &User{ID: "3", Name: "Name", Age: 1}

		err := txn.Model(user).Set(user)
		if err != nil {
			return err
		}
		return nil
	}, false)
	if err != nil {
		t.Fatal(err)
	}

	// update
	err = db.Txn(ctx, func(txn *Txn) error {
		return txn.Model("user").Update("2", Map().Set("lala", 3).Set("jj", "林俊杰"))
	}, true)
	if err != nil {
		t.Fatal(err)
	}

	// pagination
	err = db.Txn(ctx, func(txn *Txn) error {
		total, list, err := txn.Model("user").Pagination(nil, nil, 1, 10)
		if err != nil {
			return err
		}
		log.Println("total:", total)
		users := ToEntities[User](list)
		for _, user := range users {
			log.Printf("id: %s, name: %s", user.ID, user.Name)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// list
	err = db.Txn(ctx, func(txn *Txn) error {
		return txn.Model("user").List(nil, 1, func(m any, total int64) error {
			user := ToEntity[User](m)
			log.Printf("total: %d, id: %s, name: %s", total, user.ID, user.Name)
			return nil
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	// count
	err = db.Txn(ctx, func(txn *Txn) error {
		count, err := txn.Model("user").Count(nil)
		if err != nil {
			return err
		}
		log.Println("count:", count)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// inc
	err = db.Txn(ctx, func(txn *Txn) error {
		return txn.Model("user").Inc("2", "lala", -1)
	}, true)
	if err != nil {
		t.Fatal(err)
	}

	// del
	err = db.Txn(ctx, func(txn *Txn) error {
		return txn.Model("user").Del("1")
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestModelIndex(t *testing.T) {
	db := NewDatabase("mongodb://172.31.10.100:27017/?directConnection=true", "test")
	defer db.Close()

	type User struct {
		ID         string     `bson:"_id"`
		Name       string     `db:"unique"`
		Age        int64      `db:"index"`
		OrderCount string     `bson:"order_count"`
		CreatedAt  *time.Time `bson:"created_at,omitempty" db:"index"`
	}
	ctx := context.Background()

	err := db.Index(ctx, &User{})
	if err != nil {
		t.Fatal(err)
	}
}
