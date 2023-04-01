package mongo

import (
	"log"
	"testing"
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

func TestGetValueOfModelPrimaryKey(t *testing.T) {
	type User struct {
		Name       string `bson:"_id"`
		Age        int64  `db:"pk"`
		OrderCount int64
	}
	pk := GetValueOfModelPrimaryKey(&User{Name: "liran", Age: 132})
	if pk != "liran" {
		t.Fatal(pk)
	}
}
