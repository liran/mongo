// Package mongo provides utility functions for MongoDB operations.
package mongo

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/iancoleman/strcase"
	"go.mongodb.org/mongo-driver/v2/bson"
)

const indexTagName = "db"

// M is a MongoDB document with chainable field helpers.
type M bson.M

// Set assigns a field and returns the document for chaining.
func (m M) Set(key string, value any) M {
	if m == nil {
		m = make(M)
	}
	m[key] = value
	return m
}

// Unset removes a field and returns the document for chaining.
func (m M) Unset(key string) M {
	delete(m, key)
	return m
}

// Get returns a field and reports whether it exists.
func (m M) Get(key string) (any, bool) {
	value, exists := m[key]
	return value, exists
}

// NewDocument creates an empty MongoDB document.
//
// Example:
//
//	filter := mongo.NewDocument().Set("active", true)
func NewDocument() M {
	return make(M)
}

// CollectionName returns the collection name for T.
//
// Named struct types use snake_case. A pointer-receiver CollectionName method
// overrides the default. Unnamed and non-struct types return an empty string.
func CollectionName[T any]() string {
	modelType := reflect.TypeFor[T]()
	for modelType.Kind() == reflect.Pointer {
		modelType = modelType.Elem()
	}
	if modelType.Kind() != reflect.Struct || modelType.Name() == "" {
		return ""
	}

	document := reflect.New(modelType).Interface()
	if namer, ok := document.(CollectionNamer); ok {
		name := namer.CollectionName()
		if name != "" {
			return name
		}
	}
	return toSnake(modelType.Name())
}

func toSnake(text string) string {
	return strcase.ToSnakeWithIgnore(text, ".")
}

// IDFilter returns an _id equality filter.
func IDFilter(id any) bson.D {
	idElement := bson.E{Key: "_id", Value: id}
	filter := bson.D{idElement}
	return filter
}

// Ptr returns a pointer to value.
func Ptr[T any](value T) *T {
	return &value
}

// IDOf returns the top-level MongoDB _id encoded by document.
//
// Struct fields must be tagged bson:"_id". IDs inside bson:",inline" structs
// and string-keyed maps are supported. IDOf never panics for unsupported input.
func IDOf(document any) (any, bool) {
	visited := make(map[valueVisit]struct{})
	return idFromValue(reflect.ValueOf(document), visited)
}

type valueVisit struct {
	modelType reflect.Type
	pointer   uintptr
}

func idFromValue(value reflect.Value, visited map[valueVisit]struct{}) (any, bool) {
	for value.IsValid() && value.Kind() == reflect.Interface {
		if value.IsNil() {
			return nil, false
		}
		value = value.Elem()
	}

	for value.IsValid() && value.Kind() == reflect.Pointer {
		if value.IsNil() {
			return nil, false
		}
		visit := valueVisit{modelType: value.Type(), pointer: value.Pointer()}
		if _, exists := visited[visit]; exists {
			return nil, false
		}
		visited[visit] = struct{}{}
		value = value.Elem()
		for value.IsValid() && value.Kind() == reflect.Interface {
			if value.IsNil() {
				return nil, false
			}
			value = value.Elem()
		}
	}

	if !value.IsValid() {
		return nil, false
	}

	switch value.Kind() {
	case reflect.Struct:
		modelType := value.Type()
		for index := 0; index < modelType.NumField(); index++ {
			fieldType := modelType.Field(index)
			if !fieldType.IsExported() {
				continue
			}

			fieldInfo := parseBSONField(fieldType)
			if fieldInfo.skip {
				continue
			}
			fieldValue := value.Field(index)
			if fieldInfo.name == "_id" {
				return fieldValue.Interface(), true
			}
			if fieldInfo.inline {
				if id, found := idFromValue(fieldValue, visited); found {
					return id, true
				}
			}
		}
	case reflect.Map:
		keyType := value.Type().Key()
		if keyType.Kind() != reflect.String {
			return nil, false
		}
		key := reflect.New(keyType).Elem()
		key.SetString("_id")
		id := value.MapIndex(key)
		if id.IsValid() {
			return id.Interface(), true
		}
	}
	return nil, false
}

// IndexDefinition describes one index declared by model tags.
type IndexDefinition struct {
	// Name is empty for an automatically named single-field index.
	Name string

	// Keys contains ordered ascending field paths.
	Keys bson.D

	// Unique indicates whether MongoDB must enforce uniqueness.
	Unique bool
}

// IndexesFor returns the ordered index definitions declared on T.
//
// Fields support db:"index", db:"unique", db:"index=name", and
// db:"unique=name". Reusing a name builds a compound index in struct order.
// Nested structs use dotted BSON paths; bson:",inline" fields remain top-level.
func IndexesFor[T any]() ([]IndexDefinition, error) {
	modelType := reflect.TypeFor[T]()
	for modelType.Kind() == reflect.Pointer {
		modelType = modelType.Elem()
	}
	if modelType.Kind() != reflect.Struct || modelType.Name() == "" {
		return nil, ErrInvalidModelName
	}

	parser := indexParser{
		definitions: make([]IndexDefinition, 0),
		groups:      make(map[string]int),
		stack:       make(map[reflect.Type]bool),
	}
	err := parser.parseFields(modelType, "")
	if err != nil {
		return nil, err
	}
	return compactIndexDefinitions(parser.definitions)
}

type bsonFieldInfo struct {
	name   string
	inline bool
	skip   bool
}

func parseBSONField(field reflect.StructField) bsonFieldInfo {
	info := bsonFieldInfo{name: strings.ToLower(field.Name)}
	tag, exists := field.Tag.Lookup("bson")
	if !exists {
		return info
	}
	if tag == "-" {
		info.skip = true
		return info
	}

	parts := strings.Split(tag, ",")
	if parts[0] != "" {
		info.name = parts[0]
	}
	for _, option := range parts[1:] {
		if option == "inline" {
			info.inline = true
		}
	}
	return info
}

type indexTag struct {
	index      bool
	indexName  string
	unique     bool
	uniqueName string
}

func parseIndexTag(tag string) (indexTag, error) {
	result := indexTag{}
	for _, rawPart := range strings.Split(tag, ",") {
		part := strings.TrimSpace(rawPart)
		if part == "" {
			continue
		}

		pieces := strings.SplitN(part, "=", 2)
		kind := strings.ToLower(strings.TrimSpace(pieces[0]))
		name := ""
		if len(pieces) == 2 {
			name = strings.TrimSpace(pieces[1])
			if name == "" {
				return indexTag{}, fmt.Errorf("%w: %q has an empty name", ErrInvalidIndexDeclaration, part)
			}
		}

		switch kind {
		case "index":
			if result.index && result.indexName != name {
				return indexTag{}, fmt.Errorf("%w: index is declared more than once", ErrInvalidIndexDeclaration)
			}
			result.index = true
			result.indexName = name
		case "unique":
			if result.unique && result.uniqueName != name {
				return indexTag{}, fmt.Errorf("%w: unique is declared more than once", ErrInvalidIndexDeclaration)
			}
			result.unique = true
			result.uniqueName = name
		default:
			return indexTag{}, fmt.Errorf("%w: unsupported option %q", ErrInvalidIndexDeclaration, kind)
		}
	}
	if !result.index && !result.unique {
		return indexTag{}, fmt.Errorf("%w: declaration is empty", ErrInvalidIndexDeclaration)
	}
	return result, nil
}

type indexParser struct {
	definitions []IndexDefinition
	groups      map[string]int
	stack       map[reflect.Type]bool
}

func (p *indexParser) parseFields(modelType reflect.Type, prefix string) error {
	if p.stack[modelType] {
		return nil
	}
	p.stack[modelType] = true
	defer delete(p.stack, modelType)

	for index := 0; index < modelType.NumField(); index++ {
		fieldType := modelType.Field(index)
		if !fieldType.IsExported() {
			continue
		}

		fieldInfo := parseBSONField(fieldType)
		if fieldInfo.skip {
			continue
		}

		fieldPath := joinFieldPath(prefix, fieldInfo.name)
		dbTag, hasIndexTag := fieldType.Tag.Lookup(indexTagName)
		if hasIndexTag && strings.TrimSpace(dbTag) != "" {
			if fieldInfo.inline {
				return fmt.Errorf("%w: inline field %s cannot be indexed directly", ErrInvalidIndexDeclaration, fieldType.Name)
			}
			declaration, err := parseIndexTag(dbTag)
			if err != nil {
				return fmt.Errorf("field %s: %w", fieldType.Name, err)
			}
			if err := p.appendDeclarations(fieldPath, declaration); err != nil {
				return fmt.Errorf("field %s: %w", fieldType.Name, err)
			}
			continue
		}

		innerType := fieldType.Type
		for innerType.Kind() == reflect.Pointer {
			innerType = innerType.Elem()
		}
		if innerType.Kind() != reflect.Struct {
			continue
		}

		innerPrefix := fieldPath
		if fieldInfo.inline {
			innerPrefix = prefix
		}
		if err := p.parseFields(innerType, innerPrefix); err != nil {
			return err
		}
	}
	return nil
}

func (p *indexParser) appendDeclarations(fieldPath string, tag indexTag) error {
	if tag.index && tag.unique && tag.indexName == tag.uniqueName {
		return p.appendDefinition(fieldPath, tag.uniqueName, true)
	}
	if tag.index {
		if err := p.appendDefinition(fieldPath, tag.indexName, false); err != nil {
			return err
		}
	}
	if tag.unique {
		return p.appendDefinition(fieldPath, tag.uniqueName, true)
	}
	return nil
}

func (p *indexParser) appendDefinition(fieldPath, name string, unique bool) error {
	key := bson.E{Key: fieldPath, Value: 1}
	if name == "" {
		definition := IndexDefinition{Keys: bson.D{key}, Unique: unique}
		p.definitions = append(p.definitions, definition)
		return nil
	}

	definitionIndex, exists := p.groups[name]
	if !exists {
		definition := IndexDefinition{Name: name, Keys: bson.D{key}, Unique: unique}
		p.definitions = append(p.definitions, definition)
		p.groups[name] = len(p.definitions) - 1
		return nil
	}

	definition := &p.definitions[definitionIndex]
	if definition.Unique != unique {
		return fmt.Errorf("%w: index group %q mixes unique and non-unique fields", ErrInvalidIndexDeclaration, name)
	}
	for _, existingKey := range definition.Keys {
		if existingKey.Key == fieldPath {
			return fmt.Errorf("%w: index group %q repeats field %q", ErrInvalidIndexDeclaration, name, fieldPath)
		}
	}
	definition.Keys = append(definition.Keys, key)
	return nil
}

func joinFieldPath(prefix, name string) string {
	if prefix == "" {
		return name
	}
	return prefix + "." + name
}

func compactIndexDefinitions(definitions []IndexDefinition) ([]IndexDefinition, error) {
	results := make([]IndexDefinition, 0, len(definitions))
	for _, definition := range definitions {
		duplicateIndex := -1
		for index, existing := range results {
			if indexKeysEqual(existing.Keys, definition.Keys) {
				duplicateIndex = index
				break
			}
		}
		if duplicateIndex < 0 {
			results = append(results, definition)
			continue
		}

		existing := results[duplicateIndex]
		if existing.Unique == definition.Unique {
			return nil, fmt.Errorf("%w: keys %v are declared more than once", ErrInvalidIndexDeclaration, definition.Keys)
		}
		if definition.Unique {
			results[duplicateIndex] = definition
		}
	}
	return results, nil
}

func indexKeysEqual(left, right bson.D) bool {
	if len(left) != len(right) {
		return false
	}
	for index := range left {
		if left[index].Key != right[index].Key {
			return false
		}
		if fmt.Sprint(left[index].Value) != fmt.Sprint(right[index].Value) {
			return false
		}
	}
	return true
}

// Decode converts a BSON-compatible document to T.
func Decode[T any](document any) (*T, error) {
	raw, err := bson.Marshal(document)
	if err != nil {
		return nil, err
	}

	result := new(T)
	if err := bson.Unmarshal(raw, result); err != nil {
		return nil, err
	}
	return result, nil
}

// MustDecode converts a BSON-compatible document to T and panics on failure.
// Use Decode when malformed or schema-incompatible input is possible.
func MustDecode[T any](document any) *T {
	result, err := Decode[T](document)
	if err != nil {
		panic(err)
	}
	return result
}

// DecodeMany converts MongoDB documents to T and identifies a failing item.
func DecodeMany[T any](documents []M) ([]*T, error) {
	results := make([]*T, 0, len(documents))
	for index, document := range documents {
		result, err := Decode[T](document)
		if err != nil {
			return nil, fmt.Errorf("decode document %d: %w", index, err)
		}
		results = append(results, result)
	}
	return results, nil
}
