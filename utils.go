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

const TagName = "db"

type M bson.M

func (m M) Set(k string, v any) M {
	m[k] = v
	return m
}

func (m M) Del(k string) M {
	delete(m, k)
	return m
}

func (m M) Get(k string) (any, bool) {
	val, ok := m[k]
	return val, ok
}

func Map() M {
	return make(M)
}

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

func ToSnake(text string) string {
	return strcase.ToSnakeWithIgnore(text, ".")
}

func GetIdFilter(id any) any {
	return bson.D{{Key: "_id", Value: id}}
}

func Pointer[T any](v T) *T {
	return &v
}

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

// CompoundIndex represents a compound index with its fields and uniqueness
type CompoundIndex struct {
	Fields []string
	Unique bool
}

// ParseModelIndexes parses model indexes and returns detailed index information
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

		// Parse inner indexes for nested structures
		parseInnerIndex := func() {
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
		}

		if tag == "" {
			parseInnerIndex()
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

type TagInfo struct {
	Unique     bool   // unique tag is used to indicate that the field is a unique index
	UniqueName string // unique name tag is used to indicate the name of the unique index
	Index      bool   // index tag is used to indicate that the field is an index
	IndexName  string // index name tag is used to indicate the name of the index
	PrimaryKey bool   // primary key tag is used to indicate that the field is a primary key
}

// index=name,unique=name,pk
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

func ToEntities[T any](items []M) []*T {
	var os []*T
	for _, v := range items {
		os = append(os, ToEntity[T](v))
	}
	return os
}

var random = rand.New(rand.NewSource(time.Now().UnixNano()))

// RandInRange returns a random positive integer from an inclusive minimum to an exclusive maximum
func RandInRange(minInclusive, maxExclusive int) int {
	return random.Intn(maxExclusive-minInclusive) + minInclusive
}

func SequentialID() string {
	text := fmt.Sprintf("%d%d", time.Now().UTC().UnixMicro(), RandInRange(100, 1000))
	var sb strings.Builder
	for _, v := range text {
		sb.WriteRune(v + 49)
	}
	return sb.String()
}
