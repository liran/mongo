package mongo_test

import (
	"errors"
	"testing"

	"github.com/liran/mongo"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/v2/bson"
)

type utilityUser struct {
	ID   string `bson:"_id"`
	Name string `bson:"name"`
}

type utilityArchive struct {
	ID string `bson:"_id"`
}

func (*utilityArchive) CollectionName() string {
	return "archives"
}

type utilityDefaultCollection struct{}

func (*utilityDefaultCollection) CollectionName() string {
	return ""
}

type UtilityInlineID struct {
	ID string `bson:"_id"`
}

type utilityInlineDocument struct {
	*UtilityInlineID `bson:",inline"`
	Identifier       string `bson:"_identifier"`
}

type utilityNestedDocument struct {
	Child UtilityInlineID `bson:"child"`
}

type utilitySimilarIDDocument struct {
	Identifier string `bson:"_identifier"`
	ID         string `bson:"_id"`
}

type utilityIgnoredIDDocument struct {
	Child UtilityInlineID `bson:"-"`
}

type utilityInlineMapDocument struct {
	Values map[string]any `bson:",inline"`
}

type utilityRecursiveDocument struct {
	Next *utilityRecursiveDocument `bson:",inline"`
}

func TestDocumentHelpers(t *testing.T) {
	document := mongo.NewDocument().Set("name", "Liran").Set("active", true)
	name, exists := document.Get("name")
	require.True(t, exists)
	require.Equal(t, "Liran", name)

	document.Unset("active")
	_, exists = document.Get("active")
	require.False(t, exists)

	filter := mongo.IDFilter("user-1")
	require.Equal(t, bson.D{{Key: "_id", Value: "user-1"}}, filter)
	require.Equal(t, "value", *mongo.Ptr("value"))

	var nilDocument mongo.M
	nilDocument = nilDocument.Set("initialized", true)
	require.Equal(t, true, nilDocument["initialized"])
}

func TestCollectionName(t *testing.T) {
	require.Equal(t, "utility_user", mongo.CollectionName[utilityUser]())
	require.Equal(t, "utility_user", mongo.CollectionName[*utilityUser]())
	require.Equal(t, "archives", mongo.CollectionName[utilityArchive]())
	require.Equal(t, "utility_default_collection", mongo.CollectionName[utilityDefaultCollection]())
	require.Empty(t, mongo.CollectionName[map[string]any]())
}

func TestIDOf(t *testing.T) {
	t.Run("struct", func(t *testing.T) {
		user := &utilityUser{ID: "user-1"}
		id, found := mongo.IDOf(user)
		require.True(t, found)
		require.Equal(t, "user-1", id)
	})

	t.Run("exact BSON name", func(t *testing.T) {
		document := &utilitySimilarIDDocument{Identifier: "wrong", ID: "right"}
		id, found := mongo.IDOf(document)
		require.True(t, found)
		require.Equal(t, "right", id)
	})

	t.Run("inline", func(t *testing.T) {
		document := &utilityInlineDocument{
			UtilityInlineID: &UtilityInlineID{ID: "user-2"},
			Identifier:      "not-an-id",
		}
		id, found := mongo.IDOf(document)
		require.True(t, found)
		require.Equal(t, "user-2", id)
	})

	t.Run("non-inline nested ID", func(t *testing.T) {
		document := &utilityNestedDocument{Child: UtilityInlineID{ID: "nested"}}
		_, found := mongo.IDOf(document)
		require.False(t, found)
	})

	t.Run("ignored nested ID", func(t *testing.T) {
		document := &utilityIgnoredIDDocument{Child: UtilityInlineID{ID: "ignored"}}
		_, found := mongo.IDOf(document)
		require.False(t, found)
	})

	t.Run("inline map", func(t *testing.T) {
		document := &utilityInlineMapDocument{Values: map[string]any{"_id": "inline-map"}}
		id, found := mongo.IDOf(document)
		require.True(t, found)
		require.Equal(t, "inline-map", id)
	})

	t.Run("map", func(t *testing.T) {
		document := mongo.M{"_id": "map-1"}
		id, found := mongo.IDOf(document)
		require.True(t, found)
		require.Equal(t, "map-1", id)
	})

	t.Run("unsupported map key", func(t *testing.T) {
		_, found := mongo.IDOf(map[int]string{1: "value"})
		require.False(t, found)
	})

	t.Run("nil and cycle", func(t *testing.T) {
		var nilUser *utilityUser
		_, found := mongo.IDOf(nilUser)
		require.False(t, found)

		document := new(utilityRecursiveDocument)
		document.Next = document
		_, found = mongo.IDOf(document)
		require.False(t, found)

		user := &utilityUser{ID: "pointer-pointer"}
		userPointer := &user
		id, found := mongo.IDOf(userPointer)
		require.True(t, found)
		require.Equal(t, "pointer-pointer", id)
	})
}

func TestIndexesFor(t *testing.T) {
	t.Run("compound and standalone", func(t *testing.T) {
		type Job struct {
			TaskID string `bson:"task_id" db:"index,unique=job_task_url"`
			URL    string `bson:"url" db:"unique=job_task_url"`
			Status string `bson:"status" db:"index"`
		}

		definitions, err := mongo.IndexesFor[Job]()
		require.NoError(t, err)
		require.Len(t, definitions, 3)
		require.Equal(t, bson.D{{Key: "task_id", Value: 1}}, definitions[0].Keys)
		require.False(t, definitions[0].Unique)
		require.Equal(t, "job_task_url", definitions[1].Name)
		require.Equal(t, bson.D{{Key: "task_id", Value: 1}, {Key: "url", Value: 1}}, definitions[1].Keys)
		require.True(t, definitions[1].Unique)
		require.Equal(t, bson.D{{Key: "status", Value: 1}}, definitions[2].Keys)
	})

	t.Run("BSON names and nesting", func(t *testing.T) {
		type Address struct {
			PostalCode string `db:"index"`
		}
		type User struct {
			CreatedAt int     `db:"index"`
			Address   Address `bson:"address"`
			Inline    Address `bson:",inline"`
			Ignored   string  `bson:"-" db:"index"`
		}

		definitions, err := mongo.IndexesFor[User]()
		require.NoError(t, err)
		require.Len(t, definitions, 3)
		require.Equal(t, "createdat", definitions[0].Keys[0].Key)
		require.Equal(t, "address.postalcode", definitions[1].Keys[0].Key)
		require.Equal(t, "postalcode", definitions[2].Keys[0].Key)
	})

	t.Run("anonymous inline pointer", func(t *testing.T) {
		type Job struct {
			Status string `bson:"status" db:"index"`
			Group  string `bson:"group,omitempty" db:"index"`
		}
		type SearchJob struct {
			*Job      `bson:",inline"`
			TaskID    string `bson:"task_id,omitempty" db:"index"`
			Signature string `bson:"signature" db:"unique"`
		}

		// No Job value is initialized: index discovery uses type metadata.
		definitions, err := mongo.IndexesFor[SearchJob]()
		require.NoError(t, err)
		require.Len(t, definitions, 4)
		require.Equal(t, bson.D{{Key: "status", Value: 1}}, definitions[0].Keys)
		require.Equal(t, bson.D{{Key: "group", Value: 1}}, definitions[1].Keys)
		require.Equal(t, bson.D{{Key: "task_id", Value: 1}}, definitions[2].Keys)
		require.Equal(t, bson.D{{Key: "signature", Value: 1}}, definitions[3].Keys)
		require.True(t, definitions[3].Unique)
	})

	t.Run("unique subsumes same standalone index", func(t *testing.T) {
		type User struct {
			Email string `bson:"email" db:"unique,index"`
		}

		definitions, err := mongo.IndexesFor[User]()
		require.NoError(t, err)
		require.Len(t, definitions, 1)
		require.True(t, definitions[0].Unique)
		require.Equal(t, bson.D{{Key: "email", Value: 1}}, definitions[0].Keys)
	})

	t.Run("single custom name", func(t *testing.T) {
		type User struct {
			Email string `bson:"email" db:"unique=user_email"`
		}

		definitions, err := mongo.IndexesFor[User]()
		require.NoError(t, err)
		require.Len(t, definitions, 1)
		require.Equal(t, "user_email", definitions[0].Name)
	})

	t.Run("unique removes redundant regular index", func(t *testing.T) {
		type User struct {
			Email string `bson:"email" db:"index,unique=user_email"`
		}

		definitions, err := mongo.IndexesFor[User]()
		require.NoError(t, err)
		require.Len(t, definitions, 1)
		require.Equal(t, "user_email", definitions[0].Name)
		require.True(t, definitions[0].Unique)
	})

	t.Run("recursive model", func(t *testing.T) {
		type Node struct {
			Parent *Node
			Value  string `db:"INDEX"`
		}

		definitions, err := mongo.IndexesFor[Node]()
		require.NoError(t, err)
		require.Len(t, definitions, 1)
		require.Equal(t, "value", definitions[0].Keys[0].Key)
	})
}

func TestIndexesForRejectsInvalidDeclarations(t *testing.T) {
	tests := []struct {
		name string
		run  func() error
	}{
		{
			name: "mixed group types",
			run: func() error {
				type Model struct {
					First  string `db:"index=shared"`
					Second string `db:"unique=shared"`
				}
				_, err := mongo.IndexesFor[Model]()
				return err
			},
		},
		{
			name: "empty name",
			run: func() error {
				type Model struct {
					Value string `db:"index="`
				}
				_, err := mongo.IndexesFor[Model]()
				return err
			},
		},
		{
			name: "unsupported option",
			run: func() error {
				type Model struct {
					Value string `db:"pk"`
				}
				_, err := mongo.IndexesFor[Model]()
				return err
			},
		},
		{
			name: "empty declaration",
			run: func() error {
				type Model struct {
					Value string `db:",,,"`
				}
				_, err := mongo.IndexesFor[Model]()
				return err
			},
		},
		{
			name: "duplicate declaration names",
			run: func() error {
				type Model struct {
					Value string `db:"index=first,index=second"`
				}
				_, err := mongo.IndexesFor[Model]()
				return err
			},
		},
		{
			name: "duplicate keys",
			run: func() error {
				type Model struct {
					First  string `bson:"value" db:"index"`
					Second string `bson:"value" db:"index"`
				}
				_, err := mongo.IndexesFor[Model]()
				return err
			},
		},
		{
			name: "direct inline index",
			run: func() error {
				type Inner struct {
					Value string
				}
				type Model struct {
					Inner `bson:",inline" db:"index"`
				}
				_, err := mongo.IndexesFor[Model]()
				return err
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := test.run()
			require.ErrorIs(t, err, mongo.ErrInvalidIndexDeclaration)
		})
	}

	_, err := mongo.IndexesFor[map[string]any]()
	require.ErrorIs(t, err, mongo.ErrInvalidModelName)
}

func TestDecode(t *testing.T) {
	document := mongo.M{"_id": "user-1", "name": "Liran"}
	user, err := mongo.Decode[utilityUser](document)
	require.NoError(t, err)
	require.Equal(t, "user-1", user.ID)
	require.Equal(t, "Liran", user.Name)
	require.Equal(t, user, mongo.MustDecode[utilityUser](document))

	users, err := mongo.DecodeMany[utilityUser]([]mongo.M{document})
	require.NoError(t, err)
	require.Equal(t, []*utilityUser{user}, users)

	_, err = mongo.Decode[utilityUser](mongo.M{"name": make(chan int)})
	require.Error(t, err)
	require.Panics(t, func() {
		mongo.MustDecode[utilityUser](mongo.M{"name": make(chan int)})
	})

	type Number struct {
		Value int `bson:"value"`
	}
	invalidDocuments := []mongo.M{{"value": 1}, {"value": "invalid"}}
	_, err = mongo.DecodeMany[Number](invalidDocuments)
	require.Error(t, err)
	require.False(t, errors.Is(err, mongo.ErrRecordNotFound))
}
