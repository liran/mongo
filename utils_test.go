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
		Name       string `json:"name" bson:"_id,omitempty"`
		Age        int64  `json:"age" bson:"age,omitempty"`
		OrderCount int64  `json:"order_count" bson:"order_count,omitempty"`
	}
	pk := GetValueOfModelPrimaryKey(&User{Name: "liran", Age: 132})
	if pk != "liran" {
		t.Fatal(pk)
	}
}
