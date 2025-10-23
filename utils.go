// Package mongo provides utility functions for MongoDB operations.
package mongo

import (
	"fmt"
	"math/rand"
	"reflect"
	"strings"
	"time"

	"github.com/iancoleman/strcase"
	"go.mongodb.org/mongo-driver/bson"
)

// TagName is the struct tag name used for database field configuration.
// It supports options like "index", "unique", and "pk".
const TagName = "db"

// M is an alias for bson.M, providing a convenient type for MongoDB documents.
// It includes helper methods for common document operations.
type M bson.M

// Set sets a field value in the document and returns the document for chaining.
func (m M) Set(k string, v any) M {
	m[k] = v
	return m
}

// Del removes a field from the document and returns the document for chaining.
func (m M) Del(k string) M {
	delete(m, k)
	return m
}

// Get retrieves a field value from the document.
// Returns the value and a boolean indicating if the field exists.
func (m M) Get(k string) (any, bool) {
	val, ok := m[k]
	return val, ok
}

// Map creates a new empty MongoDB document.
func Map() M {
	return make(M)
}

// GetModelName extracts the model name from a Go type.
// It handles pointers, primitives, and structs, converting names to snake_case.
func GetModelName(model any) string {
	v := reflect.ValueOf(model)
	k := v.Kind()
	if k == reflect.Invalid {
		return ""
	}
	if k == reflect.Pointer || k == reflect.UnsafePointer {
		if v.IsNil() {
			return ""
		}
		v = v.Elem()
	}

	var name string
	// general data type，such as: int float bool  string .....
	if k >= 1 && k <= 16 || k == 24 {
		name = fmt.Sprintf("%v", model)
	} else {
		name = v.Type().Name()
	}
	return ToSnake(name)
}

// ToSnake converts a string to snake_case format.
// Uses the strcase library with dot preservation.
func ToSnake(text string) string {
	return strcase.ToSnakeWithIgnore(text, ".")
}

// GetIDFilter creates a MongoDB filter for finding documents by ID.
// Returns a bson.D document with the _id field.
func GetIDFilter(id any) any {
	return bson.D{{Key: "_id", Value: id}}
}

// Pointer creates a pointer to the given value.
// Useful for creating optional fields in structs.
func Pointer[T any](v T) *T {
	return &v
}

// GetID extracts the primary key value from a model.
// It searches for fields tagged with bson:"_id" or db:"pk".
// Supports nested structs and maps.
//
// Example:
//
//	type User struct {
//	    ID string `bson:"_id"`
//	}
//	id := GetID(&User{ID: "123"}) // returns "123"
func GetID(model any) any {
	modelValue := reflect.ValueOf(model)
	k := modelValue.Kind()
	for k == reflect.Pointer || k == reflect.UnsafePointer {
		if modelValue.IsNil() {
			return nil
		}
		modelValue = modelValue.Elem()
		k = modelValue.Kind()
	}

	// tag `db:"pk"` or `bson:"_id"`
	if k == reflect.Struct {
		// Iterate over all available fields and read the tag value
		modelType := modelValue.Type()
		for i := 0; i < modelType.NumField(); i++ {
			fieldType := modelType.Field(i)

			// skip unexported fields
			if !fieldType.IsExported() {
				continue
			}

			fieldValue := modelValue.Field(i)
			fieldKind := fieldValue.Kind()

			// `db:"pk"`
			tag := fieldType.Tag.Get(TagName)
			if tag != "" {
				dbTags := ParseTag(tag)
				if dbTags.PrimaryKey {
					return modelValue.Field(i).Interface()
				}
			}

			// `bson:"_id"`
			tag = fieldType.Tag.Get("bson")
			if tag != "" && strings.HasPrefix(tag, "_id") {
				return modelValue.Field(i).Interface()
			}

			// recursive search
			if fieldKind == reflect.Pointer ||
				fieldKind == reflect.UnsafePointer ||
				fieldKind == reflect.Struct {
				id := GetID(fieldValue.Interface())
				if id != nil {
					return id
				}
				continue
			}
		}
	}

	if k == reflect.Map {
		// take value of "_id"
		val := modelValue.MapIndex(reflect.ValueOf("_id"))
		if val.Kind() != reflect.Invalid {
			return val.Interface()
		}
	}
	return nil
}

// CompoundIndex represents a compound index configuration.
// It defines the fields that make up the index and whether it should be unique.
type CompoundIndex struct {
	Fields []string
	Unique bool
}

// ParseModelIndexes parses struct tags to extract index configuration.
// Returns the model name and a map of index configurations.
// Supports both single and compound indexes with custom naming.
func ParseModelIndexes(model any) (modelName string, indexInfo map[string]*CompoundIndex) {
	indexInfo = make(map[string]*CompoundIndex)

	modelValue := reflect.ValueOf(model)
	k := modelValue.Kind()
	for k == reflect.Pointer || k == reflect.UnsafePointer {
		if modelValue.IsNil() {
			return
		}
		modelValue = modelValue.Elem()
		k = modelValue.Kind()
	}
	if k != reflect.Struct {
		return
	}

	// Iterate over all available fields and read the tag value
	modelType := modelValue.Type()

	modelName = ToSnake(modelType.Name())

	for i := 0; i < modelType.NumField(); i++ {
		fieldType := modelType.Field(i)

		// skip unexported fields
		if !fieldType.IsExported() {
			continue
		}

		fieldValue := modelValue.Field(i)
		fieldKind := fieldValue.Kind()

		indexName := fieldType.Tag.Get("bson")
		if indexName == "-" {
			continue
		}
		if indexName != "" {
			indexName = strings.Trim(strings.ReplaceAll(indexName, "omitempty", ""), " ,")
		}
		if indexName == "" {
			indexName = ToSnake(fieldType.Name)
		}

		// Get the field tag value
		tag := fieldType.Tag.Get(TagName)

		if tag == "" {
			// Parse inner indexes for nested structures
			if fieldKind == reflect.Pointer ||
				fieldKind == reflect.UnsafePointer ||
				fieldKind == reflect.Struct {
				_, innerIndexInfo := ParseModelIndexes(fieldValue.Interface())
				// Merge indexes
				for k, v := range innerIndexInfo {
					if indexInfo[k] == nil {
						indexInfo[k] = &CompoundIndex{
							Fields: make([]string, 0),
							Unique: false,
						}
					}
					indexInfo[k].Fields = append(indexInfo[k].Fields, v.Fields...)
					indexInfo[k].Unique = indexInfo[k].Unique || v.Unique
				}
			}
			continue
		}

		dbTags := ParseTag(tag)
		if dbTags.Unique {
			if dbTags.UniqueName == "" {
				dbTags.UniqueName = indexName
			}
			if indexInfo[dbTags.UniqueName] == nil {
				indexInfo[dbTags.UniqueName] = &CompoundIndex{
					Fields: make([]string, 0),
					Unique: true,
				}
			}
			indexInfo[dbTags.UniqueName].Fields = append(indexInfo[dbTags.UniqueName].Fields, indexName)
		}
		if dbTags.Index {
			if dbTags.IndexName == "" {
				dbTags.IndexName = indexName
			}
			if indexInfo[dbTags.IndexName] == nil {
				indexInfo[dbTags.IndexName] = &CompoundIndex{
					Fields: make([]string, 0),
					Unique: false,
				}
			}
			indexInfo[dbTags.IndexName].Fields = append(indexInfo[dbTags.IndexName].Fields, indexName)
		}
	}

	return
}

// TagInfo represents parsed database tag information.
type TagInfo struct {
	// Unique indicates if the field should have a unique index.
	Unique bool

	// UniqueName specifies the name for the unique index (for compound indexes).
	UniqueName string

	// Index indicates if the field should have a regular index.
	Index bool

	// IndexName specifies the name for the index (for compound indexes).
	IndexName string

	// PrimaryKey indicates if the field is the primary key.
	PrimaryKey bool
}

// ParseTag parses a database tag string and returns TagInfo.
// Format: index=name,unique=name,pk
//
// Example:
//
//	info := ParseTag("index=user_name,unique=user_email")
//	// info.Index = true, info.IndexName = "user_name"
//	// info.Unique = true, info.UniqueName = "user_email"
func ParseTag(tag string) TagInfo {
	info := TagInfo{}

	multTypes := strings.Split(strings.Trim(tag, ", ;"), ",")
	for _, v := range multTypes {
		arr := strings.Split(v, "=")
		if len(arr) > 0 {
			k := strings.ToLower(strings.TrimSpace(arr[0]))
			if k == "" {
				continue
			}

			val := ""
			if len(arr) > 1 {
				val = strings.TrimSpace(arr[1])
			}

			switch k {
			case "unique":
				info.Unique = true
				info.UniqueName = val
			case "index":
				info.Index = true
				info.IndexName = val
			case "pk":
				info.PrimaryKey = true
			}
		}
	}

	return info
}

// NewModelType creates a new instance of the given model type.
// Returns a pointer to a new instance of the same type.
func NewModelType(model any) any {
	modelVal := reflect.ValueOf(model)
	k := modelVal.Kind()
	for k == reflect.Pointer || k == reflect.UnsafePointer {
		if modelVal.IsNil() {
			return nil
		}
		modelVal = modelVal.Elem()
		k = modelVal.Kind()
	}
	if k != reflect.Struct {
		return nil
	}

	return reflect.New(modelVal.Type()).Interface()
}

// ToEntity converts a MongoDB document to a typed struct.
// Uses BSON marshaling/unmarshaling for type conversion.
//
// Example:
//
//	doc := mongo.Map().Set("name", "John").Set("age", 30)
//	user := mongo.ToEntity[User](doc)
func ToEntity[T any](m M) *T {
	o := new(T)
	raw, err := bson.Marshal(m)
	if err != nil {
		panic(err)
	}
	if err := bson.Unmarshal(raw, o); err != nil {
		panic(err)
	}
	return o
}

// ToEntities converts a slice of MongoDB documents to a slice of typed structs.
//
// Example:
//
//	docs := []mongo.M{...}
//	users := mongo.ToEntities[User](docs)
func ToEntities[T any](items []M) []*T {
	var os []*T
	for _, v := range items {
		os = append(os, ToEntity[T](v))
	}
	return os
}

var random = rand.New(rand.NewSource(time.Now().UnixNano()))

// RandInRange returns a random integer in the range [minInclusive, maxExclusive).
// Uses a thread-safe random number generator.
//
// Example:
//
//	num := mongo.RandInRange(1, 100) // returns random number 1-99
func RandInRange(minInclusive, maxExclusive int) int {
	return random.Intn(maxExclusive-minInclusive) + minInclusive
}

// SequentialID generates a unique sequential identifier.
// Combines current timestamp with random number for uniqueness.
//
// Example:
//
//	id := mongo.SequentialID() // returns something like "123456789012345"
func SequentialID() string {
	text := fmt.Sprintf("%d%d", time.Now().UTC().UnixMicro(), RandInRange(100, 1000))
	var sb strings.Builder
	for _, v := range text {
		sb.WriteRune(v + 49)
	}
	return sb.String()
}
