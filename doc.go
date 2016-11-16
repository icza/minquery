/*

Package minquery provides a mgo-like Query type called MinQuery, which supports
efficient query pagination (cursors to continue listing documents where
we left off).

Example using MinQuery

Let's say we have a users collection in MongoDB modeled with this Go struct:

    type User struct {
        ID      bson.ObjectID `bson:"_id"`
        Name    string        `bson:"name"`
        Country string        `bson:"country"`
    }

To query users having country=USA, sorted by Name and ID:

    q := minquery.New(session.DB(""), "users", bson.M{"country" : "USA"}).
        Sort("name", "_id").Limit(10)
    // If this is not the first page, set cursor:
    if cursor := getLastCursor(); cursor != "" {
        q = q.Cursor(cursor)
    }

    var users []*User
    newCursor, err := q.All(&users, "country", "name", "_id")

And that's all. newCursor is the cursor to be used to fetch the next batch.

Note #1: When calling MinQuery.All(), you have to provide the names
of the cursor fields, this will be used to build the cursor data
(and ultimately the cursor string) from.

Note #2: If you're retrieving partial results (by using MinQuery.Select()),
you have to include all the fields that are part of the cursor (the index entry)
even if you don't intend to use them directly, else MinQuery.All() will not
have all the values of the cursor fields, and so it will not be able to create
the proper cursor value.

*/
package minquery
