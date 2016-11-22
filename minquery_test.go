package minquery

import (
	"fmt"
	"testing"

	mgo "gopkg.in/mgo.v2"
	"gopkg.in/mgo.v2/bson"

	"github.com/icza/mighty"
)

var sess *mgo.Session

func init() {
	var err error
	sess, err = mgo.Dial("")
	if err != nil {
		panic(err)
	}
	bi, err := sess.BuildInfo()
	fmt.Println(bi.Version, err)
}

type User struct {
	ID      bson.ObjectId `bson:"_id"`
	Name    string        `bson:"name"`
	Country string        `bson:"country"`
}

func TestMinQuery(t *testing.T) {
	eq, neq, expDeq := mighty.Eq(t), mighty.Neq(t), mighty.ExpDeq(t)
	_, _, _ = eq, neq, expDeq

	c := sess.DB("").C("users")
	_ = c
}
