package minquery

import (
	"encoding/hex"
	"log"
	"testing"

	"github.com/globalsign/mgo"
	"github.com/globalsign/mgo/bson"
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

	mgoCompany := sess.DB("").C("company")
	if err := mgoCompany.EnsureIndex(mgo.Index{Key: []string{"name", "boos"}}); err != nil {
		panic(err)
	}
	if err := mgoCompany.EnsureIndex(mgo.Index{Key: []string{"name", "boos.name"}}); err != nil {
		panic(err)
	}
	if err := mgoCompany.EnsureIndex(mgo.Index{Key: []string{"subsidiary"}}); err != nil {
		panic(err)
	}
	if err := mgoCompany.EnsureIndex(mgo.Index{Key: []string{"subsidiary.0"}}); err != nil {
		panic(err)
	}
	if _, err := mgoCompany.RemoveAll(nil); err != nil {
		panic(err)
	}
}

func clearDB() {
	c := sess.DB("")
	if _, err := c.C("users").RemoveAll(nil); err != nil {
		panic(err)
	}
	if _, err := c.C("company").RemoveAll(nil); err != nil {
		panic(err)
	}
}

type User struct {
	ID      bson.ObjectId `bson:"_id"`
	Name    string        `bson:"name"`
	Country string        `bson:"country"`
}

type Company struct {
	ID      	bson.ObjectId	`bson:"_id"`
	Name		string			`bson:"name"`
	Boos		User			`bson:"boos"`
	Subsidiary	[]string		`bson:"subsidiary"`
}

func TestMinQuery(t *testing.T) {
	clearDB()
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

	mq := NewWithHint(
		sess.DB(""),
		"users",
		bson.M{"country": "US"},
		map[string]int{"name": 1, "_id": 1},
	).
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

	_, err = mq.Cursor(cursor).All(&result, cursorFields...)
	eq(nil, err)
	eq(0, len(result))

	var parres []bson.M
	_, err = mq.Sort("+name", "-_id", "").Select(bson.M{"name": 1, "_id": 1}).
		Cursor("").Limit(2).All(&parres, cursorFields...)
	eq(nil, err)
	deq([]bson.M{
		{"name": "Alice", "_id": users[2].ID},
		{"name": "Alice", "_id": users[1].ID},
	}, parres)

	_, err = mq.CursorCodec(testCodec{}).All(&parres, cursorFields...)
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

func TestMinQueryMultiIndex(t *testing.T) {
	clearDB()
	eq, neq, deq := mighty.Eq(t), mighty.Neq(t), mighty.Deq(t)
	_, _ = eq, neq

	c := sess.DB("").C("company")

	// Insert test documents:
	companies := []*Company{
		{Name: "amazon", Boos: User{ID: bson.NewObjectId(), Name: "Dakota", Country: "US"}, Subsidiary: []string{}},
		{Name: "apple", Boos: User{ID: bson.NewObjectId(), Name: "Fae", Country: "US"}, Subsidiary: []string{}},
		{Name: "facebook", Boos: User{ID: bson.NewObjectId(), Name: "Chloe", Country: "UK"}, Subsidiary: []string{}},
		{Name: "google", Boos: User{ID: bson.NewObjectId(), Name: "Aaron", Country: "UK"}, Subsidiary: []string{}},
		{Name: "honer", Boos: User{ID: bson.NewObjectId(), Name: "Ed", Country: "US"}, Subsidiary: []string{}},
		{Name: "videos", Boos: User{ID: bson.NewObjectId(), Name: "Glan", Country: "US"}, Subsidiary: []string{}},
		{Name: "zMind", Boos: User{ID: bson.NewObjectId(), Name: "len", Country: "US"}, Subsidiary: []string{}},
	}

	cursorFields := []string{"name", "boos"}

	for _, u := range companies {
		u.ID = bson.NewObjectId()
		eq(nil, c.Insert(u))
	}

	mq := New(sess.DB(""), "company", bson.M{}).
		Sort("name", "boos").Limit(3)
	var result []*Company

	cursor, err := mq.All(&result, cursorFields...)
	eq(nil, err)
	deq(companies[0:3], result)

	cursor, err = mq.Cursor(cursor).All(&result, cursorFields...)
	eq(nil, err)
	deq(companies[3:6], result)

	cursor, err = mq.Cursor(cursor).All(&result, cursorFields...)
	eq(nil, err)
	deq(companies[6:], result)
}

func TestMinQuerySubDocIndex(t *testing.T) {
	clearDB()
	eq, neq, deq := mighty.Eq(t), mighty.Neq(t), mighty.Deq(t)
	_, _ = eq, neq

	c := sess.DB("").C("company")

	// Insert test documents:
	companies := []*Company{
		{Name: "amazon", Boos: User{ID: bson.NewObjectId(), Name: "Dakota", Country: "US"}, Subsidiary: []string{}},
		{Name: "apple", Boos: User{ID: bson.NewObjectId(), Name: "Fae", Country: "US"}, Subsidiary: []string{}},
		{Name: "facebook", Boos: User{ID: bson.NewObjectId(), Name: "Chloe", Country: "UK"}, Subsidiary: []string{}},
		{Name: "google", Boos: User{ID: bson.NewObjectId(), Name: "Aaron", Country: "UK"}, Subsidiary: []string{}},
		{Name: "honer", Boos: User{ID: bson.NewObjectId(), Name: "Ed", Country: "US"}, Subsidiary: []string{}},
		{Name: "videos", Boos: User{ID: bson.NewObjectId(), Name: "Glan", Country: "US"}, Subsidiary: []string{}},
		{Name: "zMind", Boos: User{ID: bson.NewObjectId(), Name: "len", Country: "US"}, Subsidiary: []string{}},
	}

	cursorFields := []string{"name", "boos.name"}

	for _, u := range companies {
		u.ID = bson.NewObjectId()
		eq(nil, c.Insert(u))
	}

	mq := New(sess.DB(""), "company", bson.M{}).
		Sort("name", "boos.name").Limit(3)
	var result []*Company

	cursor, err := mq.All(&result, cursorFields...)
	eq(nil, err)
	deq(companies[0:3], result)

	cursor, err = mq.Cursor(cursor).All(&result, cursorFields...)
	eq(nil, err)
	deq(companies[3:6], result)

	cursor, err = mq.Cursor(cursor).All(&result, cursorFields...)
	eq(nil, err)
	deq(companies[6:], result)
}

func TestMinQueryArrayMemberIndex(t *testing.T) {
	clearDB()
	eq, neq, deq := mighty.Eq(t), mighty.Neq(t), mighty.Deq(t)
	_, _ = eq, neq

	c := sess.DB("").C("company")

	companies := []*Company{
		{Name: "amazon", Boos: User{ID: bson.NewObjectId()}, Subsidiary: []string{"ama", "zon"}},
		{Name: "apple", Boos: User{ID: bson.NewObjectId()}, Subsidiary: []string{"app", "le"}},
		{Name: "facebook", Boos: User{ID: bson.NewObjectId()}, Subsidiary: []string{"face", "book"}},
		{Name: "google", Boos: User{ID: bson.NewObjectId()}, Subsidiary: []string{"goo", "gle"}},
		{Name: "honer", Boos: User{ID: bson.NewObjectId()}, Subsidiary: []string{"hon", "er"}},
		{Name: "videos", Boos: User{ID: bson.NewObjectId()}, Subsidiary: []string{"video", "s"}},
	}

	cursorFields := []string{"subsidiary.0"}

	for _, u := range companies {
		u.ID = bson.NewObjectId()
		eq(nil, c.Insert(u))
	}
	mq := New(sess.DB(""), "company", bson.M{}).
		Sort("subsidiary.0").Limit(3)
	var result []*Company

	cursor, err := mq.All(&result, cursorFields...)
	eq(nil, err)
	deq(companies[0:3], result)

	cursor, err = mq.Cursor(cursor).All(&result, cursorFields...)
	eq(nil, err)
	deq(companies[3:6], result)

	cursor, err = mq.Cursor(cursor).All(&result, cursorFields...)
	eq(nil, err)
	deq(companies[6:], result)
}

type testCodec struct {
	testError bool
}

func (tc testCodec) CreateCursor(cursorData bson.D) (string, error) {
	if tc.testError {
		return "", errTestValue
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
