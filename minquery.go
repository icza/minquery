// Package minquery provides an efficient mgo-like Query type that supports
// MongoDB pagination (cursors to continue listing documents where we left off).
package minquery

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

	// All retrieves all documents from the result set into the provided slice.
	// cursFields lists the fields (in order) to be used to generate
	// the returned cursor.
	All(result interface{}, cursorFields ...string) (cursor string, err error)
}
