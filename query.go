package mongo

import (
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

// Page is an offset-based query result. Items is always non-nil on success.
type Page[T any] struct {
	Items    []*T
	Total    int64
	Number   int64
	PageSize int64
}

// Query is a reusable typed query builder. Chain methods return a modified copy,
// leaving the source query unchanged.
type Query[T any] struct {
	collection *Collection[T]
	filter     any
	sort       any
	projection any
	after      any
	limit      int64
	batchSize  int
	descending bool
}

// Sort sets the MongoDB sort document. After uses _id ordering and therefore
// takes precedence over Sort.
func (q *Query[T]) Sort(sort any) *Query[T] {
	next := q.clone()
	next.sort = sort
	return next
}

// Select sets the field projection. Each requires _id to remain projected so it
// can advance between batches; MongoDB includes _id by default.
func (q *Query[T]) Select(projection any) *Query[T] {
	next := q.clone()
	next.projection = projection
	return next
}

// Limit limits the documents returned by All. Values less than one mean no limit.
func (q *Query[T]) Limit(limit int64) *Query[T] {
	next := q.clone()
	next.limit = limit
	return next
}

// After starts exclusive _id cursor pagination after id. Combined with Desc,
// it selects IDs lower than id; otherwise it selects IDs greater than id.
func (q *Query[T]) After(id any) *Query[T] {
	next := q.clone()
	next.after = id
	return next
}

// Desc uses descending _id order for After and Each. It does not replace Sort
// for ordinary queries that do not use an _id cursor.
func (q *Query[T]) Desc() *Query[T] {
	next := q.clone()
	next.descending = true
	return next
}

// Batch sets the maximum documents fetched by each request made by Each.
// Values less than one use the default batch size of 100.
func (q *Query[T]) Batch(size int) *Query[T] {
	next := q.clone()
	next.batchSize = size
	return next
}

// First returns the first matching document and applies Sort and Select.
// It returns ErrRecordNotFound when no document matches.
func (q *Query[T]) First() (*T, error) {
	findOptions := options.FindOne()
	if hasID(q.after) {
		sortOrder := 1
		if q.descending {
			sortOrder = -1
		}
		findOptions.SetSort(M{"_id": sortOrder})
	} else if q.sort != nil {
		findOptions.SetSort(q.sort)
	}
	if q.projection != nil {
		findOptions.SetProjection(q.projection)
	}

	filter := queryFilter(q)
	result := q.collection.collection.FindOne(q.collection.ctx, filter, findOptions)
	document := new(T)
	if err := result.Decode(document); err != nil {
		return nil, normalizeReadError(err)
	}
	return document, nil
}

// All returns all matching documents, subject to Sort, Select, After, and Limit.
// It returns a non-nil empty slice when no document matches.
func (q *Query[T]) All() ([]*T, error) {
	findOptions := options.Find()
	applyQueryOptions(q, findOptions)
	filter := queryFilter(q)
	cursor, err := q.collection.collection.Find(q.collection.ctx, filter, findOptions)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(q.collection.ctx)

	documents := make([]*T, 0)
	if err := cursor.All(q.collection.ctx, &documents); err != nil {
		return nil, err
	}
	return documents, nil
}

// Page returns one offset-based page and its exact total matching count.
// Page numbers less than one become 1; page sizes less than one become 20.
//
// Example:
//
//	page, err := db.Find[User](ctx, filter).Page(1, 20)
//	users := page.Items
//	total := page.Total
func (q *Query[T]) Page(number, pageSize int64) (*Page[T], error) {
	return q.collection.page(q, number, pageSize)
}

// Each traverses documents in stable _id batches without loading the complete
// result set into memory. Return false to stop successfully. The callback is
// invoked serially, and its error stops traversal immediately.
//
// Example:
//
//	err := db.Find[User](ctx, filter).Batch(500).Each(
//		func(user *User) (bool, error) {
//			return true, process(user)
//		},
//	)
func (q *Query[T]) Each(callback func(*T) (bool, error)) error {
	return q.collection.each(q, callback)
}

// Count returns the exact number of matching documents, including After when set.
func (q *Query[T]) Count() (int64, error) {
	filter := queryFilter(q)
	return q.collection.collection.CountDocuments(q.collection.ctx, filter)
}

// Exists reports whether at least one document matches, including After when set.
func (q *Query[T]) Exists() (bool, error) {
	countOptions := options.Count().SetLimit(1)
	filter := queryFilter(q)
	count, err := q.collection.collection.CountDocuments(q.collection.ctx, filter, countOptions)
	return count > 0, err
}

func queryFilter[T any](query *Query[T]) any {
	filter := normalizedFilter(query.filter)
	if !hasID(query.after) {
		return filter
	}

	comparison := "$gt"
	if query.descending {
		comparison = "$lt"
	}
	return filterAfterID(filter, comparison, query.after)
}

func applyQueryOptions[T any](query *Query[T], findOptions *options.FindOptionsBuilder) {
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
	if query.limit > 0 {
		findOptions.SetLimit(query.limit)
	}
}

func (q *Query[T]) clone() *Query[T] {
	next := *q
	result := &next
	return result
}
