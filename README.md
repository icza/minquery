# minquery

![Build Status](https://github.com/icza/minquery/actions/workflows/go.yml/badge.svg)
[![Go Reference](https://pkg.go.dev/badge/github.com/icza/minquery.svg)](https://pkg.go.dev/github.com/icza/minquery)
[![Go Report Card](https://goreportcard.com/badge/github.com/icza/minquery)](https://goreportcard.com/report/github.com/icza/minquery)
[![codecov](https://codecov.io/gh/icza/minquery/branch/master/graph/badge.svg)](https://codecov.io/gh/icza/minquery)

MongoDB / `mgo` query that supports _efficient_ pagination (cursors to continue listing documents where we left off).

**Note:** Only MongoDB 3.2 and newer versions support the feature used by this package.

**Note #2:** minquery [v1.0.0](https://github.com/icza/minquery/releases/tag/v1.0.0)
uses the `gopkg.in/mgo.v2` mgo driver which has gone unmaintained
for a long time now. minquery [v2.0.0](https://github.com/icza/minquery/releases/tag/v2.0.0)
(tip of master) uses the new, community supported fork `github.com/globalsign/mgo`.
It is highly recommended to switch over to `globalsign/mgo`. If you can't or don't
want to, you may continue to use the v1.0.0 release with `gopkg.in/mgo.v2`.

## Introduction

Let's say we have a `users` collection in MongoDB modeled with this Go `struct`:

    type User struct {
        ID      bson.ObjectId `bson:"_id"`
        Name    string        `bson:"name"`
        Country string        `bson:"country"`
    }

To achieve paging of the results of some query, MongoDB and the [`mgo`](https://godoc.org/github.com/globalsign/mgo)
driver package has built-in support in the form of [`Query.Skip()`](https://godoc.org/github.com/globalsign/mgo#Query.Skip) and [`Query.Limit()`](https://godoc.org/github.com/globalsign/mgo#Query.Limit), e.g.:

    session, err := mgo.Dial(url) // Acquire Mongo session, handle error!

    c := session.DB("").C("users")
    q := c.Find(bson.M{"country" : "USA"}).Sort("name", "_id").Limit(10)

    // To get the nth page:
    q = q.Skip((n-1)*10)

    var users []*User
    err = q.All(&users)

This however becomes slow if the page number increases, as MongoDB can't just "magically" jump to the x<sup>th</sup> document in the result, it has to iterate over all the result documents and omit (not return) the first `x` that need to be skipped.

MongoDB provides the right solution: If the query operates on an index (it has to work on an index), [`cursor.min()`](https://docs.mongodb.com/manual/reference/method/cursor.min/) can be used to specify the first _index entry_ to start listing results from.

This Stack Overflow answer shows how it can be done using a mongo client: [How to do pagination using range queries in MongoDB?](http://stackoverflow.com/questions/5525304/how-to-do-pagination-using-range-queries-in-mongodb/5526907#5526907)

Note: the required index for the above query would be:

    db.users.createIndex(
        {
            country: 1,
            name: 1,
            _id: 1
        }
    )

There is one problem though: the `mgo` package has no support specifying this `min()`.

## Introducing `minquery`

Unfortunately the [`mgo`](https://godoc.org/github.com/globalsign/mgo) driver does not provide API calls to specify [`cursor.min()`](https://docs.mongodb.com/manual/reference/method/cursor.min/).

But there is a solution. The [`mgo.Database`](https://godoc.org/github.com/globalsign/mgo#Database) type provides a [`Database.Run()`](https://godoc.org/github.com/globalsign/mgo#Database.Run) method to run any MongoDB commands. The available commands and their documentation can be found here: [Database commands](https://docs.mongodb.com/manual/reference/command/)

Starting with MongoDB 3.2, a new [`find`](https://docs.mongodb.com/manual/reference/command/find/) command is available which can be used to execute queries, and it supports specifying the `min` argument that denotes the first index entry to start listing results from.

Good. What we need to do is after each batch (documents of a page) generate the `min` document from the last document of the query result, which must contain the values of the index entry that was used to execute the query, and then the next batch (the documents of the next page) can be acquired by setting this min index entry prior to executing the query.

This index entry –let's call it _cursor_ from now on– may be encoded to a `string` and sent to the client along with the results, and when the client wants the next page, he sends back the _cursor_ saying he wants results starting after this cursor.

And this is where `minquery` comes into the picture. It provides a wrapper to configure and execute a MongoDB `find` command, allowing you to specify a cursor, and after executing the query, it gives you back the new cursor to be used to query the next batch of results. The wrapper is the [`MinQuery`](https://godoc.org/github.com/icza/minquery#MinQuery) type which is very similar to [`mgo.Query`](https://godoc.org/github.com/globalsign/mgo#Query) but it supports specifying MongoDB's `min` via the `MinQuery.Cursor()` method.

The above solution using `minquery` looks like this:

    q := minquery.New(session.DB(""), "users", bson.M{"country" : "USA"}).
        Sort("name", "_id").Limit(10)
    // If this is not the first page, set cursor:
    // getLastCursor() represents your logic how you acquire the last cursor.
    if cursor := getLastCursor(); cursor != "" {
        q = q.Cursor(cursor)
    }

    var users []*User
    newCursor, err := q.All(&users, "country", "name", "_id")

And that's all. `newCursor` is the cursor to be used to fetch the next batch.

**Note #1:** When calling `MinQuery.All()`, you have to provide the names of the cursor fields, this will be used to build the cursor data (and ultimately the cursor string) from.

**Note #2:** If you're retrieving partial results (by using `MinQuery.Select()`), you have to include all the fields that are part of the cursor (the index entry) even if you don't intend to use them directly, else `MinQuery.All()` will not have all the values of the cursor fields, and so it will not be able to create the proper cursor value.
