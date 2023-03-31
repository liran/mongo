package mongo

import (
	"context"
	"errors"
	"testing"
)

func TestDatabase(t *testing.T) {
	db := NewDatabase("mongodb://172.31.10.100:27017/", "test")
	defer db.Close()

	// type User struct {
	// 	Name       string
	// 	Age        int64
	// 	OrderCount int64
	// }

	// type Order struct {
	// 	ID int64
	// 	No string
	// }

	// user := &User{}
	// order := &Order{}

	ctx := context.Background()
	err := db.Txn(ctx, func(txn *Txn) error {
		return errors.New("1234")
	})
	if err != nil {
		t.Fatal(err)
	}
}
