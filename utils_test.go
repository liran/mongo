package mongo

import (
	"log"
	"testing"
	"time"
)

func TestGetModelName(t *testing.T) {
	m := make(map[string]string, 0)
	name := GetModelName(m)
	log.Println(name)

	name = GetModelName("NameLL")
	log.Println(name)

	type ImmP struct {
		M int
	}
	var mmm ImmP
	name = GetModelName(mmm)
	log.Println(name)

	name = GetModelName(nil)
	log.Println(name)
}

func TestGetID(t *testing.T) {
	type User struct {
		Name       string `json:"name" bson:"_id,omitempty"`
		Age        int64  `json:"age" bson:"age,omitempty"`
		OrderCount int64  `json:"order_count" bson:"order_count,omitempty"`
	}

	type Parent struct {
		*User `json:"user"`
	}

	pk := GetID(&User{Name: "liran", Age: 132})
	if pk != "liran" {
		t.Fatal(pk)
	}

	pk = GetID(&Parent{User: &User{Name: "liran", Age: 132}})
	if pk != "liran" {
		t.Fatal(pk)
	}

	m := Map().Set("_id", "1")
	pk = GetID(m)
	if pk != "1" {
		t.Fatal(pk)
	}
}

func TestSequentialID(t *testing.T) {
	for i := 0; i < 10; i++ {
		log.Println(SequentialID())
	}
}

func TestParseModelIndex(t *testing.T) {
	type User struct {
		Name       string `json:"name" bson:"_id,omitempty"`
		Age        int64  `json:"age" bson:"age,omitempty" db:"index"`
		OrderCount int64  `json:"order_count" bson:"order_count,omitempty"`
	}

	type Student struct {
		*User `json:"user"`

		Class string `json:"class" db:"index"`
	}

	type Teacher struct {
		User `json:"user"`

		Class string `json:"class" db:"index"`
	}

	name, indexes := ParseModelIndex(&Student{})
	log.Println(name, indexes)

	name, indexes = ParseModelIndex(&Student{User: &User{Name: "liran"}})
	log.Println(name, indexes)

	name, indexes = ParseModelIndex(&Teacher{})
	log.Println(name, indexes)
}

func TestPointer(t *testing.T) {
	log.Println(Pointer(time.Now()).Format(time.RFC3339))
}
