package mongo

import (
	"errors"
	"reflect"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Model struct {
	txn  *Txn
	coll *mongo.Collection
}

func (m *Model) Set(model any) error {
	id := GetID(model)
	if id == nil || id == "" {
		return ErrNoID
	}

	_, err := m.coll.ReplaceOne(m.txn.ctx, GetIdFilter(id), model, options.Replace().SetUpsert(true))
	return err
}

func (m *Model) Del(id any) error {
	_, err := m.coll.DeleteOne(m.txn.ctx, GetIdFilter(id))
	return err
}

// parameter 'update' can be a structure or a Map containing the primary key
func (m *Model) Update(update any) (newRecord M, err error) {
	id := GetID(update)
	if id == nil || id == "" {
		return nil, ErrNoID
	}

	raw, err := bson.Marshal(update)
	if err != nil {
		return nil, err
	}
	updateMap := Map()
	if err := bson.Unmarshal(raw, &updateMap); err != nil {
		return nil, err
	}

	res := m.coll.FindOneAndUpdate(m.txn.ctx, GetIdFilter(id), bson.D{{Key: "$set", Value: updateMap}})
	old := Map()
	err = res.Decode(&old)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}

	for k, v := range updateMap {
		old.Set(k, v)
	}

	return old, nil
}

func (m *Model) Inc(id, fields any) error {
	_, err := m.coll.UpdateByID(m.txn.ctx, id, bson.D{{Key: "$inc", Value: fields}})
	return err
}

func (m *Model) Get(id any, projection ...any) (M, error) {
	opt := options.FindOne()
	if len(projection) > 0 {
		opt.SetProjection(projection[0])
	}
	res := m.coll.FindOne(m.txn.ctx, GetIdFilter(id), opt)
	doc := Map()
	err := res.Decode(&doc)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}
	return doc, nil
}

func (m *Model) First(filter, sort any, projection ...any) (M, error) {
	if filter == nil {
		filter = bson.D{}
	}

	opt := options.FindOne()
	if sort != nil {
		opt.SetSort(sort)
	}
	if len(projection) > 0 {
		opt.SetProjection(projection[0])
	}

	res := m.coll.FindOne(m.txn.ctx, filter, opt)
	var v M
	err := res.Decode(&v)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}
	return v, nil
}

func (m *Model) Unmarshal(id, model any, projection ...any) error {
	opt := options.FindOne()
	if len(projection) > 0 {
		opt.SetProjection(projection[0])
	}

	res := m.coll.FindOne(m.txn.ctx, GetIdFilter(id), opt)
	err := res.Decode(model)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrRecordNotFound
		}
	}
	return err
}

func (m *Model) Count(filter any) (count int64, err error) {
	val := reflect.ValueOf(filter)
	if val.Kind() == reflect.Invalid ||
		((val.Kind() == reflect.Map ||
			val.Kind() == reflect.Slice ||
			val.Kind() == reflect.Array) &&
			val.Len() < 1) {
		return m.coll.EstimatedDocumentCount(m.txn.ctx)
	}
	return m.coll.CountDocuments(m.txn.ctx, filter)
}

func (m *Model) Has(id any) (bool, error) {
	count, err := m.coll.CountDocuments(m.txn.ctx, GetIdFilter(id), options.Count().SetLimit(1))
	return count > 0, err
}

func (m *Model) Pagination(filter, sort any, page, pageSize int64, projection ...any) (total int64, list []M, err error) {
	total, err = m.Count(filter)
	if err != nil {
		return
	}

	if total < 1 {
		return
	}

	if page < 1 {
		page = 1
	}

	if pageSize < 1 {
		pageSize = 1
	}

	opt := options.Find().SetSkip((page - 1) * pageSize).SetLimit(pageSize)
	if sort != nil {
		opt.SetSort(sort)
	}
	if len(projection) > 0 {
		opt.SetProjection(projection[0])
	}

	if filter == nil {
		filter = bson.D{}
	}
	cur, err := m.coll.Find(m.txn.ctx, filter, opt)
	if err != nil {
		return
	}
	err = cur.All(m.txn.ctx, &list)
	return
}

func (m *Model) Next(filter, sort M, lastID string, pageSize int64, projection ...any) (list []M, err error) {
	if filter == nil {
		filter = Map()
	}

	if lastID != "" {
		filter.Set("_id", Map().Set("$gt", lastID))
	}

	if pageSize < 1 {
		pageSize = 10
	}

	opt := options.Find().SetLimit(pageSize)
	if len(projection) > 0 {
		opt.SetProjection(projection[0])
	}
	if sort != nil {
		opt.SetSort(sort)
	}

	cur, err := m.coll.Find(m.txn.ctx, filter, opt)
	if err != nil {
		return nil, err
	}

	if err := cur.All(m.txn.ctx, &list); err != nil {
		return nil, err
	}

	return list, nil
}

// `cb` return `false` will stop iterate
func (m *Model) List(filter M, size int64, cb func(m M, total int64) (bool, error), projection ...any) error {
	total, err := m.Count(filter)
	if err != nil {
		return err
	}
	if total < 1 {
		return nil
	}

	nextFilter := Map()
	for k, v := range filter {
		nextFilter[k] = v
	}

	if size < 1 {
		size = 10
	}

	opt := options.Find().SetLimit(size)
	if len(projection) > 0 {
		opt.SetProjection(projection[0])
	}

	next := Map()
	for {
		cur, err := m.coll.Find(m.txn.ctx, nextFilter, opt)
		if err != nil {
			return err
		}

		var ps []M
		if err := cur.All(m.txn.ctx, &ps); err != nil {
			return err
		}
		for _, v := range ps {
			if ok, err := cb(v, total); err != nil || !ok {
				return err
			}
		}

		n := len(ps)
		if n > 0 {
			id, ok := ps[n-1].Get("_id")
			if ok {
				nextFilter.Set("_id", next.Set("$gt", id))
				continue
			}
		}

		break
	}

	return nil
}

func NewModel(txn *Txn, model any) *Model {
	modelName := GetModelName(model)
	if modelName == "" {
		panic(ErrInvalidModelName)
	}

	return &Model{txn: txn, coll: txn.db.Collection(modelName)}
}
