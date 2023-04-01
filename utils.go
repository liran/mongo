package mongo

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"github.com/iancoleman/strcase"
	"go.mongodb.org/mongo-driver/bson"
)

type Map map[string]any

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

// tag `db:"pk"`
func GetValueOfModelPrimaryKey(model any) any {
	modelValue := reflect.ValueOf(model)
	k := modelValue.Kind()
	for k == reflect.Pointer || k == reflect.UnsafePointer {
		if modelValue.IsNil() {
			return nil
		}
		modelValue = modelValue.Elem()
		k = modelValue.Kind()
	}
	if k != reflect.Struct {
		return nil
	}

	// Iterate over all available fields and read the tag value
	modelType := modelValue.Type()
	for i := 0; i < modelType.NumField(); i++ {
		fieldType := modelType.Field(i)

		// Get the field tag value
		tag := fieldType.Tag.Get("db")
		if tag == "" {
			continue
		}

		// if specified manually, use the specified name
		multTypes := strings.Split(strings.Trim(tag, ", ;"), ",")
		for _, v := range multTypes {
			if v == "pk" {
				return modelValue.Field(i).Interface()
			}
		}
	}
	return nil
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

func ToBytes(data any) []byte {
	var value []byte
	switch v := data.(type) {
	case []byte:
		value = v
	case string: // Prevent repeated double quotes in the string
		value = []byte(v)
	default:
		// no encode html tag
		buffer := &bytes.Buffer{}
		encoder := json.NewEncoder(buffer)
		encoder.SetEscapeHTML(false)
		encoder.Encode(data)
		buffer.Truncate(buffer.Len() - 1) // remove suffix "\n"
		value = buffer.Bytes()
	}
	return value
}

func ToEntity[T any](val any) *T {
	o := new(T)
	if err := bson.Unmarshal(ToBytes(val), o); err != nil {
		panic(err)
	}
	return o
}

func ToEntities[T any](items []any) []*T {
	var os []*T
	for _, v := range items {
		os = append(os, ToEntity[T](v))
	}
	return os
}
