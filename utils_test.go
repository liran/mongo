package mongo_test

import (
	"log"
	"testing"
	"time"

	"github.com/liran/mongo"
	"github.com/stretchr/testify/require"
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
		ID    string `json:"id" db:"pk"`
		*User `json:"user"`
	}

	pk := mongo.GetID(&User{Name: "liran", Age: 132})
	if pk != "liran" {
		t.Fatal(pk)
	}

	pk = mongo.GetID(&Parent{User: &User{Name: "liran", Age: 132}, ID: "123"})
	if pk != "123" {
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
		TaskID string `bson:"task_id" db:"index,unique=job_task_url"`
		URL    string `bson:"url" db:"unique=job_task_url"`
		Status string `bson:"status" db:"index"`
		Owner  string `bson:"owner" db:"unique"`
	}

	name, indexInfo := mongo.ParseModelIndexes(&Job{})
	log.Printf("Model name: %s", name)
	log.Printf("Job indexes: %+v", indexInfo)

	// Test case 2: User struct with multiple compound indexes
	type User struct {
		ID       string `bson:"_id,omitempty"`
		Email    string `bson:"email" db:"unique=user_email_domain"`
		Domain   string `bson:"domain" db:"unique=user_email_domain"`
		Username string `bson:"username" db:"index=user_name_region"`
		Region   string `bson:"region" db:"index=user_name_region"`
		Age      int    `bson:"age" db:"index"`
	}

	name, indexInfo = mongo.ParseModelIndexes(&User{})
	log.Printf("Model name: %s", name)
	log.Printf("User indexes: %+v", indexInfo)

	// Test case 3: Job struct with compound unique index
	type Teacher struct {
		// User `json:"user"` // auto parse inner indexes
		User `json:"user" db:"index=abc"`

		Class string `json:"class" db:"unique=user_email_domain"`
	}

	name, indexInfo = mongo.ParseModelIndexes(&Teacher{})
	log.Printf("Model name: %s", name)
	log.Printf("User indexes: %+v", indexInfo)
}

func TestPointer(t *testing.T) {
	log.Println(mongo.Pointer(time.Now()).Format(time.RFC3339))
}

func TestParseTag(t *testing.T) {
	tests := []struct {
		name     string
		tag      string
		expected mongo.TagInfo
	}{
		// Basic single tags
		{
			name: "empty tag",
			tag:  "",
			expected: mongo.TagInfo{
				Unique:     false,
				UniqueName: "",
				Index:      false,
				IndexName:  "",
				PrimaryKey: false,
			},
		},
		{
			name: "unique tag",
			tag:  "unique",
			expected: mongo.TagInfo{
				Unique:     true,
				UniqueName: "",
				Index:      false,
				IndexName:  "",
				PrimaryKey: false,
			},
		},
		{
			name: "index tag",
			tag:  "index",
			expected: mongo.TagInfo{
				Unique:     false,
				UniqueName: "",
				Index:      true,
				IndexName:  "",
				PrimaryKey: false,
			},
		},
		{
			name: "pk tag",
			tag:  "pk",
			expected: mongo.TagInfo{
				Unique:     false,
				UniqueName: "",
				Index:      false,
				IndexName:  "",
				PrimaryKey: true,
			},
		},
		// Named tags
		{
			name: "unique with name",
			tag:  "unique=user_email",
			expected: mongo.TagInfo{
				Unique:     true,
				UniqueName: "user_email",
				Index:      false,
				IndexName:  "",
				PrimaryKey: false,
			},
		},
		{
			name: "index with name",
			tag:  "index=user_name",
			expected: mongo.TagInfo{
				Unique:     false,
				UniqueName: "",
				Index:      true,
				IndexName:  "user_name",
				PrimaryKey: false,
			},
		},
		// Combined tags
		{
			name: "unique and index",
			tag:  "unique,index",
			expected: mongo.TagInfo{
				Unique:     true,
				UniqueName: "",
				Index:      true,
				IndexName:  "",
				PrimaryKey: false,
			},
		},
		{
			name: "unique with name and index with name",
			tag:  "unique=user_email,index=user_name",
			expected: mongo.TagInfo{
				Unique:     true,
				UniqueName: "user_email",
				Index:      true,
				IndexName:  "user_name",
				PrimaryKey: false,
			},
		},
		{
			name: "all tags combined",
			tag:  "unique=user_email,index=user_name,pk",
			expected: mongo.TagInfo{
				Unique:     true,
				UniqueName: "user_email",
				Index:      true,
				IndexName:  "user_name",
				PrimaryKey: true,
			},
		},
		// Edge cases
		{
			name: "whitespace around tags",
			tag:  " unique , index  , pk ",
			expected: mongo.TagInfo{
				Unique:     true,
				UniqueName: "",
				Index:      true,
				IndexName:  "",
				PrimaryKey: true,
			},
		},
		{
			name: "semicolon separator (not supported)",
			tag:  "unique;index;pk",
			expected: mongo.TagInfo{
				Unique:     false,
				UniqueName: "",
				Index:      false,
				IndexName:  "",
				PrimaryKey: false,
			},
		},
		{
			name: "mixed separators (semicolon ignored)",
			tag:  "unique,index;ss,pk",
			expected: mongo.TagInfo{
				Unique:     true,
				UniqueName: "",
				Index:      false,
				IndexName:  "",
				PrimaryKey: true,
			},
		},
		{
			name: "empty values in named tags",
			tag:  "unique=,index=",
			expected: mongo.TagInfo{
				Unique:     true,
				UniqueName: "",
				Index:      true,
				IndexName:  "",
				PrimaryKey: false,
			},
		},
		{
			name: "case insensitive",
			tag:  "UNIQUE,INDEX,PK",
			expected: mongo.TagInfo{
				Unique:     true,
				UniqueName: "",
				Index:      true,
				IndexName:  "",
				PrimaryKey: true,
			},
		},
		{
			name: "mixed case",
			tag:  "Unique=user_email,Index=user_name,Pk",
			expected: mongo.TagInfo{
				Unique:     true,
				UniqueName: "user_email",
				Index:      true,
				IndexName:  "user_name",
				PrimaryKey: true,
			},
		},
		{
			name: "duplicate tags",
			tag:  "unique,unique,index,index",
			expected: mongo.TagInfo{
				Unique:     true,
				UniqueName: "",
				Index:      true,
				IndexName:  "",
				PrimaryKey: false,
			},
		},
		{
			name: "unknown tags",
			tag:  "unknown,unique,other,index",
			expected: mongo.TagInfo{
				Unique:     true,
				UniqueName: "",
				Index:      true,
				IndexName:  "",
				PrimaryKey: false,
			},
		},
		{
			name: "empty segments",
			tag:  ",unique,,index,",
			expected: mongo.TagInfo{
				Unique:     true,
				UniqueName: "",
				Index:      true,
				IndexName:  "",
				PrimaryKey: false,
			},
		},
		{
			name: "whitespace in named values",
			tag:  "unique= user_email ,index= user_name ",
			expected: mongo.TagInfo{
				Unique:     true,
				UniqueName: "user_email",
				Index:      true,
				IndexName:  "user_name",
				PrimaryKey: false,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mongo.ParseTag(tt.tag)
			require.Equal(t, tt.expected, result)
		})
	}
}
