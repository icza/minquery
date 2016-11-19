// This file contains the MinQuery interface and its implementation.

package minquery

import (
	"gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"
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
	All(result interface{}, cursorFields ...string) (cursor string, err error)
}

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

// All implements MinQuery.All().
func (mq *minQuery) All(result interface{}, cursorFields ...string) (cursor string, err error) {
	if mq.cursorErr != nil {
		return "", mq.cursorErr
	}

	// Mongodb "find" reference:
	// https://docs.mongodb.com/manual/reference/command/find/

	cmd := bson.D{
		{Name: "find", Value: mq.coll},
		{Name: "limit", Value: mq.limit},
		{Name: "batchSize", Value: mq.limit},
		{Name: "singleBatch", Value: true},
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
			bson.DocElem{Name: "skip", Value: 0},
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

	if err = mq.db.Run(cmd, &res); err != nil {
		return
	}

	firstBatch := res.Cursor.FirstBatch
	if len(firstBatch) > 0 {
		if len(cursorFields) > 0 {
			// create cursor from the last document
			var doc bson.M
			if err = firstBatch[len(firstBatch)-1].Unmarshal(&doc); err != nil {
				return
			}
			cursorData := make(bson.D, len(cursorFields))
			for i, cf := range cursorFields {
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
	err = mq.db.C(mq.coll).NewIter(nil, firstBatch, 0, nil).All(result)
	return
}
