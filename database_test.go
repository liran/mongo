package mongo

import (
	"context"
	"log"
	"testing"
	"time"

	"github.com/pkg/errors"
)

func TestCRUD(t *testing.T) {
	db := NewDatabase("mongodb://172.31.9.163:27017/?directConnection=true", "mongo_test")
	defer db.Close()

	type User struct {
		ID         string `bson:"_id"`
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
	if err != nil && !errors.Is(err, ErrRecordNotFound) {
		t.Fatal(err)
	}

	// set
	err = db.Txn(ctx, func(txn *Txn) error {
		user := &User{ID: "2", Name: "Name2", Age: 2}

		err := txn.Model(user).Set(user)
		if err != nil {
			return err
		}
		return nil
	}, false)
	if err != nil {
		t.Fatal(err)
	}

	// get
	err = db.Txn(ctx, func(txn *Txn) error {
		m, err := txn.Model("user").Get("3")
		if err != nil {
			return err
		}

		user := ToEntity[User](m)
		log.Printf("%+v", user)
		return nil
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
		return txn.Model("user").List(nil, 1, func(m M, total int64) error {
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
		return txn.Model("user").Inc("2", Map().Set("lala", -1).Set("inc", 1))
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

	// first
	err = db.Txn(ctx, func(txn *Txn) error {
		res, err := txn.Model("user").First(nil, Map().Set("age", -1))
		if err != nil {
			return err
		}

		user := ToEntity[User](res)
		log.Printf("%+v", user)

		return nil
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

	err := db.Indexes(ctx, &User{})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCount(t *testing.T) {
	db := NewDatabase("mongodb://172.31.9.163:27017/?directConnection=true", "shopify")
	defer db.Close()

	uptime := time.Now()
	ctx := context.Background()
	err := db.Txn(ctx, func(txn *Txn) error {
		ok, err := txn.Model("host").Count(nil)
		log.Println(ok)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	log.Printf("time taken: %s", time.Since(uptime))
}

func TestUpdate(t *testing.T) {
	db := NewDatabase("mongodb://172.31.9.163:27017/?directConnection=true", "mongo_test")
	defer db.Close()

	type Video struct {
		UID          string     `json:"uid" bson:"_id"`
		AuthorUID    string     `json:"author_uid" bson:"author_uid" db:"index"`
		ID           string     `json:"id" bson:"id"`
		URL          string     `json:"url" bson:"url,omitempty"`
		MetadataS3   string     `json:"metadata_s3" bson:"metadata_s3,omitempty"`
		Description  string     `json:"description" bson:"description,omitempty"`
		Cover        string     `json:"cover" bson:"cover,omitempty"`
		CoverS3      string     `json:"cover_s3" bson:"cover_s3,omitempty"`
		Video        string     `json:"video,omitempty" bson:"video,omitempty"`
		VideoS3      string     `json:"video_s3,omitempty" bson:"video_s3,omitempty"`
		ShareCount   int        `json:"share_count" bson:"share_count,omitempty"`
		CommentCount int        `json:"comment_count" bson:"comment_count,omitempty"`
		PlayCount    int        `json:"play_count" bson:"play_count,omitempty"`
		CollectCount int        `json:"collect_count" bson:"collect_count,omitempty"`
		Expired      bool       `json:"expired" bson:"expired,omitempty"`
		CreatedAt    *time.Time `json:"created_at" bson:"created_at,omitempty"`

		TextInCover string   `json:"text_in_cover,omitempty" bson:"text_in_cover,omitempty"`
		Whatsapps   []string `json:"whatsapps" bson:"whatsapps,omitempty"`
		Emails      []string `json:"emails" bson:"emails,omitempty"`
	}

	video := &Video{
		UID:       "uid",
		ID:        "id",
		Video:     "123",
		Whatsapps: []string{"whatsapp1"},
		Emails:    []string{"email"},
	}

	ctx := context.Background()

	// set
	err := db.Txn(ctx, func(txn *Txn) error {
		return txn.Model(video).Set(video)
	})
	if err != nil {
		t.Fatal(err)
	}

	// update
	err = db.Txn(ctx, func(txn *Txn) error {
		update := &Video{UID: "uid"}
		update.Video = "456"
		update.AuthorUID = "author"
		update.Emails = nil
		newVideo, err := txn.Model(video).Update(update)
		if err != nil {
			return err
		}

		video = ToEntity[Video](newVideo)

		update1 := Map().Set("_id", "uid").Set("emails", nil).Set("collect_count", 200)
		newVideo, err = txn.Model(video).Update(update1)
		if err != nil {
			return err
		}

		video = ToEntity[Video](newVideo)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	err = db.Txn(ctx, func(txn *Txn) error {
		video.UID = "uid123"
		newVideo, err := txn.Model(video).Update(video)
		if err != nil {
			return err
		}

		video = ToEntity[Video](newVideo)
		return nil
	})
	if err != nil {
		log.Println(err)
	}

	err = db.Txn(ctx, func(txn *Txn) error {
		video.UID = "uid"
		t := time.Now()
		video.CreatedAt = &t
		video.Expired = true
		newVideo, err := txn.Model(video).Update(video)
		if err != nil {
			return err
		}

		video = ToEntity[Video](newVideo)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
