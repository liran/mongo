package mongo

import (
	"context"
	"errors"
	"reflect"

	"go.mongodb.org/mongo-driver/v2/bson"
	driver "go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const defaultBatchSize = 100

// CollectionNamer lets a document override its derived collection name.
type CollectionNamer interface {
	CollectionName() string
}

// Collection is a typed MongoDB collection bound to a context.
// T is the document type, not a pointer to it.
type Collection[T any] struct {
	ctx        context.Context
	collection *driver.Collection
}

// Raw returns the underlying official driver collection for unsupported or
// advanced MongoDB operations such as aggregation pipelines and change streams.
func (c *Collection[T]) Raw() *driver.Collection {
	return c.collection
}

// Save fully replaces a document by _id, inserting it when absent.
// Fields omitted from document are removed from an existing record.
func (c *Collection[T]) Save(document *T) error {
	id, found := IDOf(document)
	if !found || !hasID(id) {
		return ErrNoID
	}

	filter := IDFilter(id)
	replaceOptions := options.Replace().SetUpsert(true)
	_, err := c.collection.ReplaceOne(c.ctx, filter, document, replaceOptions)
	return normalizeWriteError(err)
}

// SaveMany fully replaces documents by _id using one bulk operation.
// All _id values are validated before the bulk command is sent.
func (c *Collection[T]) SaveMany(documents ...*T) error {
	if len(documents) == 0 {
		return nil
	}

	writes := make([]driver.WriteModel, 0, len(documents))
	for _, document := range documents {
		id, found := IDOf(document)
		if !found || !hasID(id) {
			return ErrNoID
		}

		write := driver.NewReplaceOneModel()
		write.SetFilter(IDFilter(id))
		write.SetReplacement(document)
		write.SetUpsert(true)
		writes = append(writes, write)
	}

	_, err := c.collection.BulkWrite(c.ctx, writes)
	return normalizeWriteError(err)
}

// Get retrieves a document by _id and returns ErrRecordNotFound when absent.
func (c *Collection[T]) Get(id any) (*T, error) {
	result := c.collection.FindOne(c.ctx, IDFilter(id))
	document := new(T)
	if err := result.Decode(document); err != nil {
		return nil, normalizeReadError(err)
	}
	return document, nil
}

// Update applies $set to selected fields and returns the resulting document.
// It writes zero values, ignores _id in fields, and does not insert when absent.
func (c *Collection[T]) Update(id, fields any) (*T, error) {
	update, err := makeUpdateDocument(fields)
	if err != nil {
		return nil, err
	}

	findOptions := options.FindOneAndUpdate().SetReturnDocument(options.After)
	result := c.collection.FindOneAndUpdate(c.ctx, IDFilter(id), update, findOptions)
	document := new(T)
	if err := result.Decode(document); err != nil {
		return nil, normalizeReadError(err)
	}
	return document, nil
}

// Increment atomically applies $inc and returns the resulting document.
func (c *Collection[T]) Increment(id, fields any) (*T, error) {
	incElement := bson.E{Key: "$inc", Value: fields}
	update := bson.D{incElement}
	findOptions := options.FindOneAndUpdate().SetReturnDocument(options.After)
	result := c.collection.FindOneAndUpdate(c.ctx, IDFilter(id), update, findOptions)
	document := new(T)
	if err := result.Decode(document); err != nil {
		return nil, normalizeReadError(err)
	}
	return document, nil
}

// Delete removes a document by _id. Deleting a missing document succeeds.
func (c *Collection[T]) Delete(id any) error {
	_, err := c.collection.DeleteOne(c.ctx, IDFilter(id))
	return normalizeWriteError(err)
}

// UpdateMany applies $set to every document matching a non-empty filter.
// The result is the modified count rather than the matched count.
func (c *Collection[T]) UpdateMany(filter, fields any) (int64, error) {
	if isEmptyFilter(filter) {
		return 0, ErrEmptyFilter
	}

	update, err := makeUpdateDocument(fields)
	if err != nil {
		return 0, err
	}
	result, err := c.collection.UpdateMany(c.ctx, filter, update)
	if err != nil {
		return 0, normalizeWriteError(err)
	}
	return result.ModifiedCount, nil
}

// DeleteMany removes every document matching a non-empty filter.
func (c *Collection[T]) DeleteMany(filter any) (int64, error) {
	if isEmptyFilter(filter) {
		return 0, ErrEmptyFilter
	}

	result, err := c.collection.DeleteMany(c.ctx, filter)
	if err != nil {
		return 0, normalizeWriteError(err)
	}
	return result.DeletedCount, nil
}

// Find starts a reusable query. Pass zero or one filter; omitting it matches all.
func (c *Collection[T]) Find(filter ...any) *Query[T] {
	query := &Query[T]{collection: c}
	if len(filter) > 0 {
		query.filter = filter[0]
	}
	return query
}

func (c *Collection[T]) count(filter any) (int64, error) {
	if isEmptyFilter(filter) {
		return c.collection.EstimatedDocumentCount(c.ctx)
	}
	return c.collection.CountDocuments(c.ctx, filter)
}

func (c *Collection[T]) page(query *Query[T], page, pageSize int64) (*Page[T], error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	filter := queryFilter(query)
	total, err := c.count(filter)
	if err != nil {
		return nil, err
	}

	result := &Page[T]{
		Items:    make([]*T, 0),
		Total:    total,
		Number:   page,
		PageSize: pageSize,
	}
	if total == 0 {
		return result, nil
	}

	findOptions := options.Find().SetSkip((page - 1) * pageSize).SetLimit(pageSize)
	if hasID(query.after) {
		sortOrder := 1
		if query.descending {
			sortOrder = -1
		}
		findOptions.SetSort(M{"_id": sortOrder})
	} else if query.sort != nil {
		findOptions.SetSort(query.sort)
	}
	if query.projection != nil {
		findOptions.SetProjection(query.projection)
	}
	cursor, err := c.collection.Find(c.ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(c.ctx)

	if err := cursor.All(c.ctx, &result.Items); err != nil {
		return nil, err
	}
	return result, nil
}

func (c *Collection[T]) each(query *Query[T], callback func(*T) (bool, error)) error {
	batchSize := query.batchSize
	if batchSize < 1 {
		batchSize = defaultBatchSize
	}

	baseFilter := queryFilter(query)
	filter := baseFilter
	sortOrder := 1
	comparison := "$gt"
	if query.descending {
		sortOrder = -1
		comparison = "$lt"
	}
	findOptions := options.Find().SetLimit(int64(batchSize))
	findOptions.SetSort(M{"_id": sortOrder})
	if query.projection != nil {
		findOptions.SetProjection(query.projection)
	}

	for {
		cursor, err := c.collection.Find(c.ctx, filter, findOptions)
		if err != nil {
			return err
		}

		count, lastID, continues, consumeErr := c.consumeBatch(cursor, callback)
		closeErr := cursor.Close(c.ctx)
		if consumeErr != nil {
			return consumeErr
		}
		if closeErr != nil {
			return closeErr
		}
		if !continues || count < batchSize {
			return nil
		}
		if !hasID(lastID) {
			return ErrNoID
		}

		filter = filterAfterID(baseFilter, comparison, lastID)
	}
}

func (c *Collection[T]) consumeBatch(cursor *driver.Cursor, callback func(*T) (bool, error)) (int, any, bool, error) {
	count := 0
	var lastID any
	for cursor.Next(c.ctx) {
		document := new(T)
		if err := cursor.Decode(document); err != nil {
			return count, lastID, false, err
		}

		continues, err := callback(document)
		if err != nil || !continues {
			return count, lastID, false, err
		}
		lastID, _ = IDOf(document)
		count++
	}
	if err := cursor.Err(); err != nil {
		return count, lastID, false, err
	}
	return count, lastID, true, nil
}

func newCollection[T any](database *Database, ctx context.Context, name string) *Collection[T] {
	if name == "" {
		panic(ErrInvalidModelName)
	}

	collection := &Collection[T]{ctx: ctx, collection: database.Database.Collection(name)}
	return collection
}

func makeUpdateDocument(fields any) (bson.D, error) {
	raw, err := bson.Marshal(fields)
	if err != nil {
		return nil, err
	}

	fieldMap := M{}
	if err := bson.Unmarshal(raw, &fieldMap); err != nil {
		return nil, err
	}
	delete(fieldMap, "_id")
	setElement := bson.E{Key: "$set", Value: fieldMap}
	update := bson.D{setElement}
	return update, nil
}

func normalizedFilter(filter any) any {
	if isEmptyFilter(filter) {
		return bson.D{}
	}
	return filter
}

func isEmptyFilter(filter any) bool {
	if filter == nil {
		return true
	}

	value := reflect.ValueOf(filter)
	switch value.Kind() {
	case reflect.Array, reflect.Map, reflect.Slice, reflect.String:
		return value.Len() == 0
	case reflect.Interface, reflect.Pointer:
		return value.IsNil()
	default:
		return false
	}
}

func hasID(id any) bool {
	if id == nil {
		return false
	}
	value := reflect.ValueOf(id)
	for value.Kind() == reflect.Interface {
		if value.IsNil() {
			return false
		}
		value = value.Elem()
	}
	switch value.Kind() {
	case reflect.Pointer:
		return !value.IsNil()
	case reflect.String:
		return value.Len() > 0
	default:
		return true
	}
}

func filterAfterID(filter any, comparison string, id any) any {
	idFilter := M{"_id": M{comparison: id}}
	if isEmptyFilter(filter) {
		return idFilter
	}
	combined := bson.D{{Key: "$and", Value: bson.A{filter, idFilter}}}
	return combined
}

func normalizeReadError(err error) error {
	if errors.Is(err, driver.ErrNoDocuments) {
		return errors.Join(ErrRecordNotFound, err)
	}
	return normalizeWriteError(err)
}

func normalizeWriteError(err error) error {
	if driver.IsDuplicateKeyError(err) {
		return errors.Join(ErrDuplicateKey, err)
	}
	return err
}
