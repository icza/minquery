package minquery

import (
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
		{Name: "Bob", Country: "US"},
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
}
