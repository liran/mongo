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

	user := &User{Name: "liran", Age: 132}
	pk := mongo.GetID(user)
	if pk != "liran" {
		t.Fatal(pk)
	}

	parentUser := &User{Name: "liran", Age: 132}
	parent := &Parent{User: parentUser, ID: "123"}
	pk = mongo.GetID(parent)
	if pk != "123" {
		t.Fatal(pk)
	}

	m := mongo.Map().Set("_id", "1")
	pk = mongo.GetID(m)
	if pk != "1" {
		t.Fatal(pk)
	}
}

func TestMapHelpers(t *testing.T) {
	document := mongo.Map().Set("name", "Liran").Set("active", true)
	name, exists := document.Get("name")
	require.True(t, exists)
	require.Equal(t, "Liran", name)

	document.Del("active")
	_, exists = document.Get("active")
	require.False(t, exists)
}

func TestEntityConversions(t *testing.T) {
	type User struct {
		ID   string `bson:"_id"`
		Name string `bson:"name"`
	}

	document := mongo.M{"_id": "user-1", "name": "Liran"}
	user := mongo.ToEntity[User](document)
	require.Equal(t, "user-1", user.ID)
	require.Equal(t, "Liran", user.Name)

	users := mongo.ToEntities[User]([]mongo.M{document})
	require.Len(t, users, 1)
	require.Equal(t, user, users[0])
}

func TestNewModelType(t *testing.T) {
	type User struct {
		ID string
	}

	user := &User{ID: "user-1"}
	created := mongo.NewModelType(user)
	require.IsType(t, new(User), created)
	require.Empty(t, created.(*User).ID)

	var nilUser *User
	require.Nil(t, mongo.NewModelType(nilUser))
	require.Nil(t, mongo.NewModelType(1))
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

	student := new(Student)
	name, indexes := mongo.ParseModelIndexes(student)
	log.Println(name, indexes)

	student = &Student{User: new(User)}
	name, indexes = mongo.ParseModelIndexes(student)
	log.Println(name, indexes)

	student2 := &Student2{User: new(User)}
	name, indexes = mongo.ParseModelIndexes(student2)
	log.Println(name, indexes)

	teacher := new(Teacher)
	name, indexes = mongo.ParseModelIndexes(teacher)
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

	job := new(Job)
	name, indexInfo := mongo.ParseModelIndexes(job)
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

	user := new(User)
	name, indexInfo = mongo.ParseModelIndexes(user)
	log.Printf("Model name: %s", name)
	log.Printf("User indexes: %+v", indexInfo)

	// Test case 3: Job struct with compound unique index
	type Teacher struct {
		// User `json:"user"` // auto parse inner indexes
		User `json:"user" db:"index=abc"`

		Class string `json:"class" db:"unique=user_email_domain"`
	}

	teacher := new(Teacher)
	name, indexInfo = mongo.ParseModelIndexes(teacher)
	log.Printf("Model name: %s", name)
	log.Printf("User indexes: %+v", indexInfo)
}

func TestParseModelIndexesFromType(t *testing.T) {
	type Base struct {
		Email string `bson:"email" db:"unique"`
	}
	type User struct {
		*Base
		Parent *User
		Age    int `bson:"age" db:"index"`
	}

	user := new(User)
	name, indexes := mongo.ParseModelIndexes(user)
	require.Equal(t, "user", name)
	require.Equal(t, []string{"email"}, indexes["email"].Fields)
	require.True(t, indexes["email"].Unique)
	require.Equal(t, []string{"age"}, indexes["age"].Fields)
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
