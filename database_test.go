package mongo

import (
	"context"
	"log"
	"testing"
)

func TestDatabase(t *testing.T) {
	db := NewDatabase("mongodb://172.31.10.100:27017/?directConnection=true", "test")
	defer db.Close()

	type User struct {
		ID         string `db:"pk" bson:"_id"`
		Name       string
		Age        int64
		OrderCount string
	}

	// type Doc struct {
	// 	No string
	// }

	user := &User{ID: "2", Name: "Name", Age: 1}
	// doc := &Doc{No: "222"}

	ctx := context.Background()

	// get not found
	// err := db.Txn(ctx, func(txn *Txn) error {
	// 	a := &User{}
	// 	err := txn.Model(user).Unmarshal("1", a)
	// 	return err
	// }, false)
	// if err != nil && !errors.Is(err, ErrNotFoundModel) {
	// 	t.Fatal(err)
	// }

	// set
	err := db.Txn(ctx, func(txn *Txn) error {
		err := txn.Model(user).Set(user)
		if err != nil {
			return err
		}

		var u User
		err = txn.Model(user).Unmarshal(user.ID, &u)
		if err != nil {
			return err
		}
		log.Printf("%+v", u)

		return nil
	}, true)
	if err != nil {
		t.Fatal(err)
	}
}
