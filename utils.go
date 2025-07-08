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

const Tag = "db"

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
			fieldValue := modelValue.Field(i)
			fieldKind := fieldValue.Kind()

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

			// `db:"pk"`
			tag := fieldType.Tag.Get(Tag)
			if tag != "" {
				dbTags := ParseTag(tag)
				_, hasPrimaryKey := dbTags["pk"]
				if hasPrimaryKey {
					return modelValue.Field(i).Interface()
				}
			}

			// `bson:"_id"`
			tag = fieldType.Tag.Get("bson")
			if tag != "" && strings.HasPrefix(tag, "_id") {
				return modelValue.Field(i).Interface()
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

func ParseModelIndex(model any) (modelName string, indexes map[string]bool) {
	indexes = make(map[string]bool)

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
		fieldValue := modelValue.Field(i)
		fieldKind := fieldValue.Kind()

		// skip unexported fields
		if !fieldType.IsExported() {
			continue
		}

		indexName := fieldType.Tag.Get("bson")
		if indexName == "-" {
			continue
		}
		if indexName != "" {
			indexName = strings.Trim(strings.ReplaceAll(indexName, "omitempty", ""), " ,")
		}
		if indexName == "" {
			indexName = strings.ToLower(fieldType.Name)
		}

		// recursive search
		if fieldKind == reflect.Pointer ||
			fieldKind == reflect.UnsafePointer ||
			fieldKind == reflect.Struct {
			_, innerIndexes := ParseModelIndex(fieldValue.Interface())
			for k, v := range innerIndexes {
				indexes[k] = v
			}
			continue
		}

		// Get the field tag value
		tag := fieldType.Tag.Get(Tag)
		if tag == "" {
			continue
		}

		dbTags := ParseTag(tag)
		_, hasIndex := dbTags["index"]
		_, hasUnique := dbTags["unique"]
		if !hasIndex && !hasUnique {
			continue
		}

		indexes[indexName] = hasUnique
	}

	return
}

func ParseTag(tag string) map[string]string {
	m := make(map[string]string)

	multTypes := strings.Split(strings.Trim(tag, ", ;"), ",")
	for _, v := range multTypes {
		arr := strings.Split(v, "=")
		if len(arr) > 0 {
			k := strings.TrimSpace(arr[0])
			if k == "" {
				continue
			}

			val := ""
			if len(arr) > 1 {
				val = arr[1]
			}
			m[strings.ToLower(k)] = val
		}
	}

	return m
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
