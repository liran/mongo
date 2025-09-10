package mongo_test

import (
	"log"
	"testing"
	"time"

	"github.com/liran/mongo"
)

func TestGetModelName(t *testing.T) {
	m := make(map[string]string, 0)
	name := mongo.GetModelName(m)
	log.Println(name)

	name = mongo.GetModelName("NameLL")
	log.Println(name)

	type ImmP struct {
		M int
	}
	var mmm ImmP
	name = mongo.GetModelName(mmm)
	log.Println(name)

	name = mongo.GetModelName(nil)
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

	pk := mongo.GetID(&User{Name: "liran", Age: 132})
	if pk != "liran" {
		t.Fatal(pk)
	}

	pk = mongo.GetID(&Parent{User: &User{Name: "liran", Age: 132}})
	if pk != "liran" {
		t.Fatal(pk)
	}

	m := mongo.Map().Set("_id", "1")
	pk = mongo.GetID(m)
	if pk != "1" {
		t.Fatal(pk)
	}
}

func TestSequentialID(t *testing.T) {
	for i := 0; i < 10; i++ {
		log.Println(mongo.SequentialID())
	}
}

func TestParseModelIndexes(t *testing.T) {
	type User struct {
		Name       string `json:"name" bson:"_id,omitempty"`
		Age        int64  `json:"age" bson:"age,omitempty" db:"unique"`
		OrderCount int64  `json:"order_count" bson:"order_count,omitempty"`
	}

	type Student struct {
		*User `json:"user" bson:"-"`

		Class string `json:"class" db:"index"`
	}

	type Student2 struct {
		*User `json:"user"`

		Class string `json:"class" db:"index"`
	}

	type Teacher struct {
		User `json:"user"`

		Class string `json:"class" db:"index"`
	}

	name, indexes := mongo.ParseModelIndexes(&Student{})
	log.Println(name, indexes)

	name, indexes = mongo.ParseModelIndexes(&Student{User: &User{}})
	log.Println(name, indexes)

	name, indexes = mongo.ParseModelIndexes(&Student2{User: &User{}})
	log.Println(name, indexes)

	name, indexes = mongo.ParseModelIndexes(&Teacher{})
	log.Println(name, indexes)
}

func TestParseModelIndexesDetailed(t *testing.T) {
	// Test case 1: Job struct with compound unique index
	type Job struct {
		TaskID string `bson:"task_id" db:"unique,group=job_task_url"`
		URL    string `bson:"url" db:"unique,group=job_task_url"`
		Status string `bson:"status" db:"index"`
		Owner  string `bson:"owner" db:"unique"`
	}

	name, indexInfo := mongo.ParseModelIndexes(&Job{})
	log.Printf("Model name: %s", name)
	log.Printf("Single indexes: %+v", indexInfo.SingleIndexes)
	log.Printf("Compound indexes: %+v", indexInfo.CompoundIndexes)

	// Verify single indexes
	if len(indexInfo.SingleIndexes) != 2 {
		t.Errorf("Expected 2 single indexes, got %d", len(indexInfo.SingleIndexes))
	}
	if indexInfo.SingleIndexes["status"] {
		t.Error("Expected status field to have unique=false")
	}
	if !indexInfo.SingleIndexes["owner"] {
		t.Error("Expected owner field to have unique=true")
	}

	// Verify compound indexes
	if len(indexInfo.CompoundIndexes) != 1 {
		t.Errorf("Expected 1 compound index, got %d", len(indexInfo.CompoundIndexes))
	}
	compoundIndex, exists := indexInfo.CompoundIndexes["job_task_url"]
	if !exists {
		t.Error("Expected compound index 'job_task_url' to exist")
	}
	if !compoundIndex.Unique {
		t.Error("Expected compound index to be unique")
	}
	if len(compoundIndex.Fields) != 2 {
		t.Errorf("Expected 2 fields in compound index, got %d", len(compoundIndex.Fields))
	}

	// Check if both fields are in the compound index
	fields := make(map[string]bool)
	for _, field := range compoundIndex.Fields {
		fields[field] = true
	}
	if !fields["task_id"] || !fields["url"] {
		t.Error("Expected both task_id and url fields in compound index")
	}

	// Test case 2: User struct with multiple compound indexes
	type User struct {
		ID       string `bson:"_id,omitempty"`
		Email    string `bson:"email" db:"unique,group=user_email_domain"`
		Domain   string `bson:"domain" db:"unique,group=user_email_domain"`
		Username string `bson:"username" db:"index,group=user_name_region"`
		Region   string `bson:"region" db:"index,group=user_name_region"`
		Age      int    `bson:"age" db:"index"`
	}

	name, indexInfo = mongo.ParseModelIndexes(&User{})
	log.Printf("Model name: %s", name)
	log.Printf("Single indexes: %+v", indexInfo.SingleIndexes)
	log.Printf("Compound indexes: %+v", indexInfo.CompoundIndexes)

	// Verify single indexes
	if len(indexInfo.SingleIndexes) != 1 {
		t.Errorf("Expected 1 single index, got %d", len(indexInfo.SingleIndexes))
	}
	if indexInfo.SingleIndexes["age"] {
		t.Error("Expected age field to have unique=false")
	}

	// Verify compound indexes
	if len(indexInfo.CompoundIndexes) != 2 {
		t.Errorf("Expected 2 compound indexes, got %d", len(indexInfo.CompoundIndexes))
	}

	// Check user_email_domain compound index
	emailDomainIndex, exists := indexInfo.CompoundIndexes["user_email_domain"]
	if !exists {
		t.Error("Expected compound index 'user_email_domain' to exist")
	}
	if !emailDomainIndex.Unique {
		t.Error("Expected user_email_domain compound index to be unique")
	}

	// Check user_name_region compound index
	nameRegionIndex, exists := indexInfo.CompoundIndexes["user_name_region"]
	if !exists {
		t.Error("Expected compound index 'user_name_region' to exist")
	}
	if nameRegionIndex.Unique {
		t.Error("Expected user_name_region compound index to be non-unique")
	}
}

func TestPointer(t *testing.T) {
	log.Println(mongo.Pointer(time.Now()).Format(time.RFC3339))
}
