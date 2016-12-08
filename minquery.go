// This file contains the MinQuery interface and its implementation.

package minquery

import (
	"errors"

	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
	"reflect"
)

// DefaultCursorCodec is the default CursorCodec value that is used if none
// is specified. The default implementation produces web-safe cursor strings.
var DefaultCursorCodec cursorCodec

// MinQuery is an mgo-like Query that supports cursors to continue listing documents
// where we left off. If a cursor is set, it specifies the last index entry
// that was already returned, and result documents will be listed after this.
type MinQuery interface {
	// Sort asks the database to order returned documents according to
	// the provided field names.
	Sort(fields ...string) MinQuery

	// Select enables selecting which fields should be retrieved for
	// the results found.
	Select(selector interface{}) MinQuery

	// Limit restricts the maximum number of documents retrieved to n,
	// and also changes the batch size to the same value.
	Limit(n int) MinQuery

	// Cursor sets the cursor, which specifies the last index entry
	// that was already returned, and result documents will be listed after this.
	// Parsing a cursor may fail which is not returned. If an invalid cursor
	// is specified, All() will fail and return the error.
	Cursor(c string) MinQuery

	// CursorCoded sets the CursorCodec to be used to parse and to create cursors.
	// This gives you the possibility to implement your own logic to create cursors,
	// including encryption should you need it.
	CursorCodec(cc CursorCodec) MinQuery

	// All retrieves all documents from the result set into the provided slice.
	// cursorFields lists the fields (in order) to be used to generate
	// the returned cursor.
	All(result interface{}, cursorFields ...string) (cursor string, err error, hasMore bool)
}

// testErrValue is the error value returned for testing purposes.
var testErrValue = errors.New("Intentional testing error")

// minQuery is the MinQuery implementation.
type minQuery struct {
	// db is the mgo Database to use
	db *mgo.Database

	// Name of the collection
	coll string

	// filter document (query)
	filter interface{}

	// sort document
	sort bson.D

	// projection document (to retrieve only selected fields)
	projection interface{}

	// limit is the max number of results
	limit int

	// Cursor, need to store and supply it if query returns no results
	cursor string

	// cursorCodec to be used to parse and to create cursors
	cursorCodec CursorCodec

	// cursorErr contains an error if an invalid cursor is supplied
	cursorErr error

	// min specifies the last index entry
	min bson.D

	// testError is a helper field to aid testing errors to reach 100% coverage.
	// May only be changed from tests! Zero value means normal operation.
	testError bool
}

// New returns a new MinQuery.
func New(db *mgo.Database, coll string, query interface{}) MinQuery {
	return &minQuery{
		db:          db,
		coll:        coll,
		filter:      query,
		cursorCodec: DefaultCursorCodec,
	}
}

// Sort implements MinQuery.Sort().
func (mq *minQuery) Sort(fields ...string) MinQuery {
	mq.sort = make(bson.D, 0, len(fields))
	for _, field := range fields {
		if field == "" {
			continue
		}
		n := 1
		if field[0] == '+' {
			field = field[1:]
		} else if field[0] == '-' {
			n, field = -1, field[1:]
		}
		mq.sort = append(mq.sort, bson.DocElem{Name: field, Value: n})
	}
	return mq
}

// Select implements MinQuery.Select().
func (mq *minQuery) Select(selector interface{}) MinQuery {
	mq.projection = selector
	return mq
}

// Limit implements MinQuery.Limit().
func (mq *minQuery) Limit(n int) MinQuery {
	mq.limit = n
	return mq
}

// Cursor implements MinQuery.Cursor().
func (mq *minQuery) Cursor(c string) MinQuery {
	mq.cursor = c
	if c != "" {
		mq.min, mq.cursorErr = mq.cursorCodec.ParseCursor(c)
	} else {
		mq.min, mq.cursorErr = nil, nil
	}
	return mq
}

// CursorCodec implements MinQuery.CursorCodec().
func (mq *minQuery) CursorCodec(cc CursorCodec) MinQuery {
	mq.cursorCodec = cc
	return mq
}

// Copied from mgo.Iter.All and edited
func allButLast(iter *mgo.Iter, result interface{}) error {
	resultv := reflect.ValueOf(result)
	if resultv.Kind() != reflect.Ptr || resultv.Elem().Kind() != reflect.Slice {
		panic("result argument must be a slice address")
	}
	slicev := resultv.Elem()
	slicev = slicev.Slice(0, slicev.Cap())
	elemt := slicev.Type().Elem()
	i := 0
	for {
		if slicev.Len() == i {
			elemp := reflect.New(elemt)
			if !iter.Next(elemp.Interface()) {
				break
			}
			slicev = reflect.Append(slicev, elemp.Elem())
			slicev = slicev.Slice(0, slicev.Cap())
		} else {
			if !iter.Next(slicev.Index(i).Addr().Interface()) {
				break
			}
		}
		i++
	}
	if i > 0 {
		resultv.Elem().Set(slicev.Slice(0, i - 1))
	} else {
		resultv.Elem().Set(slicev.Slice(0, i))
	}
	return iter.Close()
}

// All implements MinQuery.All().
func (mq *minQuery) All(result interface{}, cursorFields ...string) (cursor string, err error, hasMore bool) {
	if mq.cursorErr != nil {
		return "", mq.cursorErr, false
	}

	// Mongodb "find" reference:
	// https://docs.mongodb.com/manual/reference/command/find/

	cmd := bson.D{
		{Name: "find", Value: mq.coll},
		{Name: "singleBatch", Value: true},
	}

	queryLimit := mq.limit
	limitedQuery := (queryLimit > 0)
	if limitedQuery {
		queryLimit++ // query one more element
		cmd = append(cmd, bson.DocElem{Name: "limit", Value: queryLimit})
		cmd = append(cmd, bson.DocElem{Name: "batchSize", Value: queryLimit})
	}
	if mq.filter != nil {
		cmd = append(cmd, bson.DocElem{Name: "filter", Value: mq.filter})
	}
	if mq.sort != nil {
		cmd = append(cmd, bson.DocElem{Name: "sort", Value: mq.sort})
	}
	if mq.projection != nil {
		cmd = append(cmd, bson.DocElem{Name: "projection", Value: mq.projection})
	}
	if mq.min != nil {
		// min is inclusive, skip the first (which is the previous last)
		cmd = append(cmd,
			bson.DocElem{Name: "skip", Value: 1},
			bson.DocElem{Name: "min", Value: mq.min},
		)
	}

	var res struct {
		OK       int `bson:"ok"`
		WaitedMS int `bson:"waitedMS"`
		Cursor   struct {
			ID         interface{} `bson:"id"`
			NS         string      `bson:"ns"`
			FirstBatch []bson.Raw  `bson:"firstBatch"`
		} `bson:"cursor"`
	}

	hasMore = false
	if err = mq.db.Run(cmd, &res); err != nil {
		return
	}

	firstBatch := res.Cursor.FirstBatch
	if len(firstBatch) > 0 {
		// if return result num equal with queryLimit when limited query, there is more
		hasMore = (limitedQuery && len(firstBatch) >= queryLimit)
		if len(cursorFields) > 0 {
			offset := 0
			if hasMore {
				offset = 1
			}
			// create cursor from the last document
			var doc bson.M
			err = firstBatch[len(firstBatch) - 1 - offset].Unmarshal(&doc)
			if mq.testError {
				err = testErrValue
			}
			if err != nil {
				return
			}
			cursorData := make(bson.D, len(cursorFields))
			for i, cfd := range cursorFields {
				cf := cfd
				if '-' == cf[0] || '+' == cf[0] {
					cf = cf[1:]
				}
				cursorData[i] = bson.DocElem{Name: cf, Value: doc[cf]}
			}
			cursor, err = mq.cursorCodec.CreateCursor(cursorData)
			if err != nil {
				return
			}
		}
	} else {
		// No more results. Use the same cursor that was used for the query.
		// It's possible that the last doc was returned previously, and there
		// are no more.
		cursor = mq.cursor
	}

	// Unmarshal results (FirstBatch) into the user-provided value:
	if hasMore {
		err = allButLast(mq.db.C(mq.coll).NewIter(nil, firstBatch, 0, nil), result)
	} else {
		err = mq.db.C(mq.coll).NewIter(nil, firstBatch, 0, nil).All(result)
	}
	return
}

