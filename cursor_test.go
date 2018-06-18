package minquery

import (
	"testing"
	"time"

	"github.com/globalsign/mgo/bson"
	"github.com/icza/mighty"
)

func TestDefaultCodec(t *testing.T) {
	eq, neq, expDeq := mighty.Eq(t), mighty.Neq(t), mighty.ExpDeq(t)

	cc := cursorCodec{}

	cd := bson.D{
		{Name: "a", Value: 1},
		{Name: "b", Value: "2"},
		{Name: "c", Value: time.Date(3, 0, 0, 0, 0, 0, 0, time.UTC)},
	}
	cursor, err := cc.CreateCursor(cd)
	eq(nil, err)

	expDeq(cd)(cc.ParseCursor(cursor))

	_, err = cc.ParseCursor("%^&")
	neq(nil, err)
	_, err = cc.ParseCursor("ValidBas64ButInvalidCursor")
	neq(nil, err)
}
