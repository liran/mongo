package mongo

import (
	"encoding/json"
	"math"

	"github.com/pkg/errors"
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
	txn *Txn
	*mongo.Collection
}

func (m *Model) Set(model any, id ...any) error {
	var key any
	if len(id) > 0 {
		key = id[0]
	} else {
		key = GetValueOfModelPrimaryKey(model)
	}
	if key == nil {
		return ErrNotFoundPrimaryKey
	}

	_, err := m.ReplaceOne(m.txn.ctx, GetIdFilter(key), model, DefaultSetOption)
	return err
}

func (m *Model) Del(id any) error {
	_, err := m.DeleteOne(m.txn.ctx, GetIdFilter(id))
	return err
}

func (m *Model) Update(id, update any) error {
	_, err := m.UpdateByID(m.txn.ctx, id, bson.D{{Key: "$set", Value: update}})
	return err
}

func (m *Model) Get(id any) ([]byte, error) {
	res := m.FindOne(m.txn.ctx, GetIdFilter(id))
	raw, err := res.DecodeBytes()
	if err != nil && errors.Is(err, mongo.ErrNoDocuments) {
		return nil, ErrNotFoundModel
	}
	return raw, nil
}

func (m *Model) Unmarshal(id, model any) error {
	raw, err := m.Get(id)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, model)
}

func (m *Model) Pagination(filter, sort any, page, pageSize int64) (total int64, list []any, err error) {
	total, err = m.CountDocuments(m.txn.ctx, filter)
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
	cur, err := m.Find(m.txn.ctx, filter, opt)
	if err != nil {
		return
	}

	err = cur.All(m.txn.ctx, &list)
	return
}

func (m *Model) List(filter any, concurrency int, cb func(m any) error) error {
	if filter == nil {
		filter = bson.D{}
	}
	total, err := m.CountDocuments(m.txn.ctx, filter)
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
			cur, err := m.Find(m.txn.ctx, filter, opt)
			if err != nil {
				return err
			}

			var ps []any
			if err := cur.All(m.txn.ctx, &ps); err != nil {
				return err
			}
			for _, v := range ps {
				if err = cb(v); err != nil {
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
		panic(ErrInvalidModel)
	}

	return &Model{txn: txn, Collection: txn.db.Collection(modelName)}
}
