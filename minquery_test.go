package minquery

import (
	"encoding/hex"
	"log"
	"testing"

	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"

	"github.com/icza/mighty"
)

var sess *mgo.Session

func init() {
	var err error
	sess, err = mgo.Dial("mongodb://localhost/minquery")
	if err != nil {
		panic(err)
	}

	// Check required min version (3.2)
	bi, err := sess.BuildInfo()
	if err != nil {
		panic(err)
	}
	if !bi.VersionAtLeast(3, 2) {
		log.Panicf("This test requires at least MongoDB 3.2, got only %v", bi.Version)
	}

	c := sess.DB("").C("users")
	if err := c.EnsureIndex(mgo.Index{Key: []string{"name", "_id"}}); err != nil {
		panic(err)
	}
	if err := c.EnsureIndex(mgo.Index{Key: []string{"name", "-_id"}}); err != nil {
		panic(err)
	}
	if _, err := c.RemoveAll(nil); err != nil {
		panic(err)
	}
}

type User struct {
	ID      bson.ObjectId `bson:"_id"`
	Name    string        `bson:"name"`
	Country string        `bson:"country"`
}

func TestMinQuery(t *testing.T) {
	eq, neq, deq := mighty.Eq(t), mighty.Neq(t), mighty.Deq(t)
	_, _ = eq, neq

	c := sess.DB("").C("users")

	// Insert test documents:
	users := []*User{
		{Name: "Aaron", Country: "UK"},
		{Name: "Alice", Country: "US"},
		{Name: "Alice", Country: "US"},
		{Name: "Chloe", Country: "US"},
		{Name: "Dakota", Country: "US"},
		{Name: "Ed", Country: "US"},
		{Name: "Fae", Country: "US"},
		{Name: "Glan", Country: "US"},
	}

	cursorFields := []string{"name", "_id"}

	for _, u := range users {
		u.ID = bson.NewObjectId()
		eq(nil, c.Insert(u))
	}

	mq := New(sess.DB(""), "users", bson.M{"country": "US"}).
		Sort("name", "_id").Limit(3)

	var result []*User

	cursor, err := mq.All(&result, cursorFields...)
	eq(nil, err)
	deq(users[1:4], result)

	cursor, err = mq.Cursor(cursor).All(&result, cursorFields...)
	eq(nil, err)
	deq(users[4:7], result)

	cursor, err = mq.Cursor(cursor).All(&result, cursorFields...)
	eq(nil, err)
	deq(users[7:], result)

	cursor, err = mq.Cursor(cursor).All(&result, cursorFields...)
	eq(nil, err)
	eq(0, len(result))

	var parres []bson.M
	cursor, err = mq.Sort("+name", "-_id", "").Select(bson.M{"name": 1, "_id": 1}).
		Cursor("").Limit(2).All(&parres, cursorFields...)
	eq(nil, err)
	deq([]bson.M{
		{"name": "Alice", "_id": users[2].ID},
		{"name": "Alice", "_id": users[1].ID},
	}, parres)

	cursor, err = mq.CursorCodec(testCodec{}).All(&parres, cursorFields...)
	eq(nil, err)
	deq([]bson.M{
		{"name": "Alice", "_id": users[2].ID},
		{"name": "Alice", "_id": users[1].ID},
	}, parres)

	// Test cursor error:
	_, err = mq.Cursor("(INVALID)").All(&parres, cursorFields...)
	neq(nil, err)
	mq.Cursor("")

	// Test db.Run() failure
	mq.Select("invalid-select-doc")
	_, err = mq.All(&result, cursorFields...)
	neq(nil, err)
	mq.Select(nil)

	// Test cursor creation error
	_, err = mq.CursorCodec(testCodec{testError: true}).
		All(&parres, cursorFields...)
	neq(nil, err)

	// Test first batch unmarshal error:
	mq.(*minQuery).testError = true
	_, err = mq.CursorCodec(testCodec{testError: true}).
		All(&parres, cursorFields...)
	neq(nil, err)
	mq.(*minQuery).testError = false
}

type testCodec struct {
	testError bool
}

func (tc testCodec) CreateCursor(cursorData bson.D) (string, error) {
	if tc.testError {
		return "", testErrValue
	}
	data, err := bson.Marshal(cursorData)
	return hex.EncodeToString(data), err
}

func (testCodec) ParseCursor(c string) (cursorData bson.D, err error) {
	var data []byte
	if data, err = hex.DecodeString(c); err != nil {
		return
	}
	err = bson.Unmarshal(data, &cursorData)
	return
}
