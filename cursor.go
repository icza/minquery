// This file contains the CursorCodec interface and a default implementation.

package minquery

import (
	"encoding/base64"

	"github.com/globalsign/mgo/bson"
)

// CursorCodec represents a symmetric pair of functions that can be used to
// convert cursor data of type bson.D to a string and vice versa.
type CursorCodec interface {
	// CreateCursor returns a cursor string from the specified fields.
	CreateCursor(cursorData bson.D) (string, error)

	// ParseCursor parses the cursor string and returns the cursor data.
	ParseCursor(c string) (cursorData bson.D, err error)
}

// cursorCodec is a default implementation of CursorCodec which produces
// web-safe cursor strings by first marshaling the cursor data using
// bson.Marshal(), then using base64.RawURLEncoding.
type cursorCodec struct{}

// CreateCursor implements CursorCodec.CreateCursor().
// The returned cursor string is web-safe, and so it's safe to include
// in URL queries without escaping.
func (cursorCodec) CreateCursor(cursorData bson.D) (string, error) {
	// bson.Marshal() never returns error, so I skip a check and early return
	// (but I do return the error if it would ever happen)
	data, err := bson.Marshal(cursorData)
	return base64.RawURLEncoding.EncodeToString(data), err
}

// ParseCursor implements CursorCodec.ParseCursor().
func (cursorCodec) ParseCursor(c string) (cursorData bson.D, err error) {
	var data []byte
	if data, err = base64.RawURLEncoding.DecodeString(c); err != nil {
		return
	}

	err = bson.Unmarshal(data, &cursorData)
	return
}
