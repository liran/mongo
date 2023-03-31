package mongo

import (
	"context"

	"github.com/pkg/errors"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Txn struct {
	ctx context.Context
	db  *Database
}

func (txn *Txn) Set(model any, id ...any) error {
	modelName := GetModelName(model)
	if modelName == "" {
		return ErrInvalidModel
	}

	opts := &options.ReplaceOptions{
		BypassDocumentValidation: Pointer(true),
		Upsert:                   Pointer(true),
	}

	var key any
	if len(id) > 0 {
		key = id[0]
	} else {
		key = GetValueOfModelPrimaryKey(model)
	}
	if key == nil {
		return ErrNotFoundPrimaryKey
	}

	_, err := txn.db.Collection(modelName).ReplaceOne(txn.ctx, GetIdFilter(key), model, opts)
	return err
}

func (txn *Txn) Del(model any, id ...any) error {
	modelName := GetModelName(model)
	if modelName == "" {
		return ErrInvalidModel
	}

	var key any
	if len(id) > 0 {
		key = id[0]
	} else {
		key = GetValueOfModelPrimaryKey(model)
	}
	if key == nil {
		return ErrNotFoundPrimaryKey
	}

	_, err := txn.db.Collection(modelName).DeleteOne(txn.ctx, GetIdFilter(key))
	return err
}

func (txn *Txn) Update(model any, update any, id ...any) error {
	modelName := GetModelName(model)
	if modelName == "" {
		return ErrInvalidModel
	}

	var key any
	if len(id) > 0 {
		key = id[0]
	} else {
		key = GetValueOfModelPrimaryKey(model)
	}
	if key == nil {
		return ErrNotFoundPrimaryKey
	}

	_, err := txn.db.Collection(modelName).UpdateByID(txn.ctx, key, Map{"$set": update})
	return err
}

func (txn *Txn) Get(model any, id ...any) (any, error) {
	modelName := GetModelName(model)
	if modelName == "" {
		return nil, ErrInvalidModel
	}

	var key any
	if len(id) > 0 {
		key = id[0]
	} else {
		key = GetValueOfModelPrimaryKey(model)
	}
	if key == nil {
		return nil, ErrNotFoundPrimaryKey
	}

	alloc := NewModel(model)
	err := txn.Unmarshal(alloc, key)
	return alloc, err
}

func (txn *Txn) Unmarshal(model any, id ...any) error {
	modelName := GetModelName(model)
	if modelName == "" {
		return ErrInvalidModel
	}

	var key any
	if len(id) > 0 {
		key = id[0]
	} else {
		key = GetValueOfModelPrimaryKey(model)
	}
	if key == nil {
		return ErrNotFoundPrimaryKey
	}

	res := txn.db.Collection(modelName).FindOne(txn.ctx, GetIdFilter(key))
	err := res.Decode(model)
	if err != nil && errors.Is(err, mongo.ErrNoDocuments) {
		return ErrNotFoundModel
	}
	return err
}

func (txn *Txn) List(model, filter, sort any, page, pageSize int64) (total int64, list []any, err error) {
	modelName := GetModelName(model)
	if modelName == "" {
		err = ErrInvalidModel
		return
	}

	coll := txn.db.Collection(modelName)

	total, err = coll.CountDocuments(txn.ctx, filter)
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

	opt := options.Find().SetSort(sort).SetSkip((page - 1) * pageSize).SetLimit(pageSize)
	cur, err := coll.Find(txn.ctx, filter, opt)
	if err != nil {
		return
	}

	err = cur.All(txn.ctx, &list)
	return
}
