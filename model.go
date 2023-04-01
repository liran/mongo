package mongo

import (
	"errors"
	"math"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"golang.org/x/sync/errgroup"
)

var DefaultSetOption = &options.ReplaceOptions{
	BypassDocumentValidation: Pointer(true),
	Upsert:                   Pointer(true),
}

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

func (m *Model) Unmarshal(id, model any) error {
	res := m.coll.FindOne(m.txn.ctx, GetIdFilter(id))
	err := res.Decode(model)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return ErrNotFound
		}
		return err
	}
	return nil
}

func (m *Model) Pagination(filter, sort any, page, pageSize int64) (total int64, list []any, err error) {
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

func (m *Model) List(filter any, concurrency int, cb func(m any, total int64) error) error {
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

	var eg errgroup.Group
	eg.SetLimit(concurrency)
	handle := func(page int64) {
		eg.Go(func() error {
			opt := options.Find().SetSkip((page - 1) * pageSize).SetLimit(pageSize)
			cur, err := m.coll.Find(m.txn.ctx, filter, opt)
			if err != nil {
				return err
			}

			var ps []any
			if err := cur.All(m.txn.ctx, &ps); err != nil {
				return err
			}
			for _, v := range ps {
				if err = cb(v, total); err != nil {
					return err
				}
			}
			return nil
		})
	}

	var page int64
	pages := int64(math.Ceil(float64(total) / float64(pageSize)))
	for page = 1; page <= pages; page++ {
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
