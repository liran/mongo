package mongo

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"

	"github.com/iancoleman/strcase"
)

type Map map[string]any

func GetModelName(model any) string {
	modelVal := reflect.ValueOf(model)
	k := modelVal.Kind()
	for k == reflect.Pointer || k == reflect.UnsafePointer {
		if modelVal.IsNil() {
			return ""
		}
		modelVal = modelVal.Elem()
		k = modelVal.Kind()
	}
	if k != reflect.Struct {
		return ""
	}

	return ToSnake(modelVal.Type().Name())
}

func ToSnake(text string) string {
	return strcase.ToSnakeWithIgnore(text, ".")
}

func GetIdFilter(id any) Map {
	return Map{"_id": id}
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

func NewModel(model any) any {
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

func ToEntities[T any](items []any) []*T {
	var os []*T
	for _, v := range items {
		o := new(T)
		if err := json.Unmarshal(ToBytes(v), o); err != nil {
			panic(err)
		}
		os = append(os, o)
	}
	return os
}
