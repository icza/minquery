/*

Package minquery provides an efficient mgo-like Query type that supports
MongoDB query pagination (cursors to continue listing documents where
we left off).

Example using MinQuery

Let's say we have a users collection in MongoDB modeled with this Go `struct`:

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

Note that when calling MinQuery.All(), you have to provide the name
of the cursor fields, this will be used to build the cursor data
(and ultimately the cursor string) from.

*/
package minquery
