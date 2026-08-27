package mongo_test

import (
	"context"
	"log"

	"github.com/liran/mongo"
)

type exampleUser struct {
	ID     string `bson:"_id"`
	Name   string `bson:"name,omitempty"`
	Active bool   `bson:"active"`
	Score  int64  `bson:"score,omitempty"`
}

type exampleAccount struct {
	ID      string `bson:"_id"`
	Balance int64  `bson:"balance"`
}

type exampleIndexedJob struct {
	ID     string `bson:"_id"`
	TaskID string `bson:"task_id" db:"index,unique=task_url"`
	URL    string `bson:"url" db:"unique=task_url"`
}

func openExampleDatabase() *mongo.Database {
	db, err := mongo.Open("mongodb://localhost:27017", "example")
	if err != nil {
		log.Fatal(err)
	}
	return db
}

func ExampleDatabase_Save() {
	db := openExampleDatabase()
	defer db.Close()

	ctx := context.Background()
	user := &exampleUser{ID: "user-1", Name: "Liran", Active: true}
	if err := db.Save(ctx, user); err != nil {
		log.Fatal(err)
	}
}

func ExampleDatabase_Get() {
	db := openExampleDatabase()
	defer db.Close()

	ctx := context.Background()
	user, err := db.Get[exampleUser](ctx, "user-1")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("loaded %s", user.Name)
}

func ExampleDatabase_Update() {
	db := openExampleDatabase()
	defer db.Close()

	ctx := context.Background()
	fields := mongo.M{"active": false, "name": "new name"}
	user, err := db.Update[exampleUser](ctx, "user-1", fields)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("updated %s", user.ID)
}

func ExampleDatabase_Find() {
	db := openExampleDatabase()
	defer db.Close()

	ctx := context.Background()
	filter := mongo.M{"active": true}
	sort := mongo.M{"score": -1}
	users, err := db.Find[exampleUser](ctx, filter).
		Sort(sort).
		Limit(20).
		All()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("loaded %d users", len(users))
}

func ExampleQuery_Page() {
	db := openExampleDatabase()
	defer db.Close()

	ctx := context.Background()
	filter := mongo.M{"active": true}
	page, err := db.Find[exampleUser](ctx, filter).Page(1, 20)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%d total, %d returned", page.Total, len(page.Items))
}

func ExampleQuery_Each() {
	db := openExampleDatabase()
	defer db.Close()

	ctx := context.Background()
	filter := mongo.M{"active": true}
	err := db.Find[exampleUser](ctx, filter).BatchSize(500).Each(
		func(user *exampleUser) (bool, error) {
			log.Printf("processing %s", user.ID)
			return true, nil
		},
	)
	if err != nil {
		log.Fatal(err)
	}
}

func ExampleDatabase_Collection() {
	db := openExampleDatabase()
	defer db.Close()

	ctx := context.Background()
	archive := db.Collection[exampleUser](ctx, "archived_users")
	user, err := archive.Get("user-1")
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("loaded %s", user.Name)
}

func ExampleDatabase_EnsureIndexes() {
	db := openExampleDatabase()
	defer db.Close()

	ctx := context.Background()
	if err := db.EnsureIndexes[exampleIndexedJob](ctx); err != nil {
		log.Fatal(err)
	}
}

func ExampleDatabase_Transaction() {
	db := openExampleDatabase()
	defer db.Close()

	ctx := context.Background()
	user := &exampleUser{ID: "user-1", Active: true}
	accountFields := mongo.M{"balance": 0}
	err := db.Transaction(ctx, func(tx *mongo.Tx) error {
		if err := tx.Save(user); err != nil {
			return err
		}
		_, err := tx.Update[exampleAccount]("account-1", accountFields)
		return err
	})
	if err != nil {
		log.Fatal(err)
	}
}

func ExampleCollectionName() {
	name := mongo.CollectionName[exampleUser]()
	log.Printf("collection: %s", name)
}

func ExampleIDOf() {
	user := &exampleUser{ID: "user-1"}
	id, found := mongo.IDOf(user)
	log.Printf("id: %v, found: %t", id, found)
}

func ExampleDecode() {
	document := mongo.M{"_id": "user-1", "name": "Liran"}
	user, err := mongo.Decode[exampleUser](document)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("decoded %s", user.Name)
}

func ExampleIndexesFor() {
	definitions, err := mongo.IndexesFor[exampleIndexedJob]()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("declared indexes: %d", len(definitions))
}
