package mongo

import (
	"errors"
	"math"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.uber.org/atomic"
	"golang.org/x/sync/errgroup"
)

var DefaultSetOption = options.Replace().SetUpsert(true).SetBypassDocumentValidation(true)

type Model struct {
	txn  *Txn
	coll *mongo.Collection
}

func (m *Model) Set(model any, id ...any) error {
	var key any
	if len(id) > 0 {
		key = id[0]
	} else if pk := GetValueOfModelPrimaryKey(model); pk != "" {
		key = pk
	}

	if key == nil || key == "" {
		return ErrNoID
	}

	_, err := m.coll.ReplaceOne(m.txn.ctx, GetIdFilter(key), model, DefaultSetOption)
	return err
}

func (m *Model) Del(id any) error {
	_, err := m.coll.DeleteOne(m.txn.ctx, GetIdFilter(id))
	return err
}

func (m *Model) Update(id, update any) error {
	_, err := m.coll.UpdateByID(m.txn.ctx, id, bson.D{{Key: "$set", Value: update}})
	return err
}

func (m *Model) Inc(id, fields any) error {
	_, err := m.coll.UpdateByID(m.txn.ctx, id, bson.D{{Key: "$inc", Value: fields}})
	return err
}

func (m *Model) Get(id any) (M, error) {
	res := m.coll.FindOne(m.txn.ctx, GetIdFilter(id))
	doc := Map()
	err := res.Decode(&doc)
	return doc, err
}

func (m *Model) First(filter, sort any) (M, error) {
	if filter == nil {
		filter = bson.D{}
	}
	opt := options.FindOne()
	if sort != nil {
		opt.SetSort(sort)
	}
	res := m.coll.FindOne(m.txn.ctx, filter, opt)
	var v M
	err := res.Decode(&v)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, ErrNotFoundModel
		}
		return nil, err
	}
	return v, nil
}

func (m *Model) Unmarshal(id, model any) error {
	res := m.coll.FindOne(m.txn.ctx, GetIdFilter(id))
	err := res.Decode(model)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrNotFoundModel
		}
	}
	return err
}

func (m *Model) Count(filter any) (count int64, err error) {
	if filter == nil {
		filter = bson.D{}
	}
	return m.coll.CountDocuments(m.txn.ctx, filter)
}

func (m *Model) Has(id any) (bool, error) {
	count, err := m.coll.CountDocuments(m.txn.ctx, GetIdFilter(id), options.Count().SetLimit(1))
	return count > 0, err
}

func (m *Model) Pagination(filter, sort any, page, pageSize int64) (total int64, list []M, err error) {
	if filter == nil {
		filter = bson.D{}
	}

	total, err = m.coll.CountDocuments(m.txn.ctx, filter)
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
	cur, err := m.coll.Find(m.txn.ctx, filter, opt)
	if err != nil {
		return
	}
	err = cur.All(m.txn.ctx, &list)
	return
}

func (m *Model) List(filter any, concurrency int, cb func(m M, total int64) error) error {
	if filter == nil {
		filter = bson.D{}
	}
	total, err := m.coll.CountDocuments(m.txn.ctx, filter)
	if err != nil {
		return err
	}
	if total < 1 {
		return nil
	}

	var pageSize int64 = 200

	var stopLoop atomic.Bool
	var eg errgroup.Group
	eg.SetLimit(concurrency)
	handle := func(page int64) {
		eg.Go(func() error {
			opt := options.Find().SetSkip((page - 1) * pageSize).SetLimit(pageSize)
			cur, err := m.coll.Find(m.txn.ctx, filter, opt)
			if err != nil {
				return err
			}

			var ps []M
			if err := cur.All(m.txn.ctx, &ps); err != nil {
				return err
			}
			for _, v := range ps {
				if err = cb(v, total); err != nil {
					stopLoop.Store(true)
					return err
				}
			}
			return nil
		})
	}

	var page int64
	pages := int64(math.Ceil(float64(total) / float64(pageSize)))
	for page = 1; page <= pages; page++ {
		if stopLoop.Load() {
			break
		}
		handle(page)
	}

	return eg.Wait()
}

func NewModel(txn *Txn, model any) *Model {
	modelName := GetModelName(model)
	if modelName == "" {
		panic(ErrInvalidModelName)
	}

	return &Model{txn: txn, coll: txn.db.Collection(modelName)}
}
