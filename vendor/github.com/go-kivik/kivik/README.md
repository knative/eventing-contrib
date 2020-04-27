[![Build Status](https://gitlab.com/go-kivik/kivik/badges/master/pipeline.svg)](https://gitlab.com/go-kivik/kivik/pipelines) [![Codecov](https://img.shields.io/codecov/c/github/go-kivik/kivik.svg?style=flat)](https://codecov.io/gh/go-kivik/kivik) [![Go Report Card](https://goreportcard.com/badge/github.com/go-kivik/kivik)](https://goreportcard.com/report/github.com/go-kivik/kivik) [![GoDoc](https://godoc.org/github.com/go-kivik/kivik?status.svg)](http://godoc.org/github.com/go-kivik/kivik) [![Website](https://img.shields.io/website-up-down-green-red/http/kivik.io.svg?label=website&colorB=007fff)](http://kivik.io)

# Kivik

Package kivik provides a common interface to CouchDB or CouchDB-like databases.

The kivik package must be used in conjunction with a database driver.

The kivik driver system is modeled after the standard library's [sql](https://golang.org/pkg/database/sql/)
and [sql/driver](https://golang.org/pkg/database/sql/driver/) packages, although
the client API is completely different due to the different database models
implemented by SQL and NoSQL databases such as CouchDB.

# Versions

You are browsing the **development** branch of Kivik. The stable version is
available [here](https://github.com/go-kivik/kivik/tree/stable1.x). Please
consult the documentation on that page for proper installation of the stable
branch.

This branch which will eventually become the Kivik 2.0.0 release. The API is
subject to rapid and unannounced changes at this stage of development. For
production work, you are encouraged to use the latest 1.x release of Kivik,
which is stable.

Example configuration for common dependency managers follow.

## [dep](https://github.com/golang/dep)

Update your `Gopkg.toml` file:

```toml
[[constraint]]
  name = "github.com/go-kivik/kivik"
  branch = "master"

[[constraint]]
  name = "github.com/go-kivik/couchdb"
  branch = "master"
```

## [Glide](https://github.com/Masterminds/glide)

Update your `glide.yaml` file:

```yaml
import:
- package: github.com/go-kivik/kivik
  version: master
- package: github.com/go-kivik/couchdb
  version: master
```

## [govendor](https://github.com/kardianos/govendor)

Fetch the latest development version of Kivik and the CouchDB driver with the
following command:

        govendor fetch github.com/go-kivik/kivik
        govendor fetch github.com/go-kivik/couchdb

# Installation

Install Kivik as you normally would for any Go package:

    go get -u github.com/go-kivik/kivik
    go get -u github.com/go-kivik/couchdb

This will install the main Kivik package and the CouchDB database driver. See
the [list of Kivik database drivers](https://github.com/go-kivik/kivik/wiki/Kivik-database-drivers)
for a complete list of available drivers.

# Example Usage

Please consult the the [package documentation](https://godoc.org/github.com/go-kivik/kivik)
for all available API methods, and a complete usage documentation.  And for
additional usage examples, [consult the wiki](https://github.com/go-kivik/kivik/wiki/Usage-Examples).

```go
package main

import (
    "context"
    "fmt"

    // "github.com/flimzy/kivik" // Stable version of Kivik
    "github.com/go-kivik/kivik" // Development version of Kivik
    _ "github.com/go-kivik/couchdb" // The CouchDB driver
)

func main() {
    client, err := kivik.New("couch", "http://localhost:5984/")
    if err != nil {
        panic(err)
    }

    db, err := client.DB(context.TODO(), "animals")
    if err != nil {
        panic(err)
    }

    doc := map[string]interface{}{
        "_id":      "cow",
        "feet":     4,
        "greeting": "moo",
    }

    rev, err := db.Put(context.TODO(), "cow", doc)
    if err != nil {
        panic(err)
    }
    fmt.Printf("Cow inserted with revision %s\n", rev)
}
```

# Frequently Asked Questions

Nobody has ever asked me any of these questions, so they're probably better called
"Never Asked Questions" or possibly "Imagined Questions."

## Why another CouchDB client API?

Read the [design goals](https://github.com/go-kivik/kivik/wiki/Design-goals) for
the general design goals.

Specifically, I was motivated to write Kivik for a few reasons:

1. I was unhappy with any of the existing CouchDB drivers for Go. The [best
one](https://github.com/fjl/go-couchdb) had a number of shortcomings:

    - It is no longer actively developed.
    - It [doesn't have an open source license](https://github.com/fjl/go-couchdb/issues/15).
    - It doesn't support iterating over result sets, forcing one to load all
      results of a query into memory at once.
    - It [doesn't support CouchDB 2.0](https://github.com/fjl/go-couchdb/issues/14)
      sequence IDs or MongoDB-style queries.
    - It doesn't natively support CookieAuth (it does allow a generic Auth method
      which could be used to do this, but I think it's appropriate to put directly
      in the library).

2. I wanted a single client API that worked with both CouchDB and
[PouchDB](https://pouchdb.com/). I had previously written
[go-pouchdb](https://github.com/flimzy/go-pouchdb), a GopherJS wrapper around
the PouchDB library with a public API modeled after `fjl/go-couchdb`, but I
still wanted a unified driver infrastructure.

3. I want an unambiguous, open source license. This software is released under
the Apache 2.0 license. See the included LICENSE.md file for details.

4. I wanted the ability to mock CouchDB connections for testing. This is possible
with the `sql` / `sql/driver` approach by implementing a mock driver, but was
not possible with any existing CouchDB client libraries. This library makes that
possible for CouchDB apps, too.

5. I wanted a simple, mock CouchDB server I could use for testing. It doesn't
need to be efficient, or support all CouchDB servers, but it should be enough
to test the basic functionality of a PouchDB app, for instance. Kivik aims to
do this with the `kivik serve` command, in the near future.

6. I wanted a toolkit that would make it easy to build a proxy to sit in front
of CouchDB to handle custom authentication or other logic that CouchDB cannot
support natively. Kivik aims to accomplish this in the future.

## What are Kivik's requirements?

Kivik's test suite is automatically run on Linux and OSX for every pull request,
but should work on all supported Go platforms.

Below are the compatibility targets for specific runtime and database versions.
If you discover a bug affecting any of these supported environments, please let
me know by submitting a bug report via GitHub.

- **Go** Kivik 2.x aims for full compatibility with all stable releases of Go
  from 1.9. For Go 1.7 or 1.8 you can use [Kivik 1.x](https://github.com/go-kivik/kivik/tree/stable1.x)
- **CouchDB** The Kivik 1.x CouchDB driver aims for compatibility with all
  stable releases of CouchDB from 1.6.1.
- **GopherJS** GopherJS always requires the latest stable version of Go, so
  building Kivik with GopherJS has this same requirement.
- **PouchDB** The Kivik 1.x PouchDB driver aims for compatibility with all
  stable releases of PouchDB from 6.0.0.

## What is the development status?

Kivik 1.x is considered production-ready and comes with a complete client API
client and backend drivers for CouchDB and PouchDB.

Kivik 2.x is under development, and adds new features and a streamlined API.

Future goals are to flesh out the Memory driver, which will make automated
testing without a real CouchDB server easier. Then I will work on completing
the 'serve' mode.

You can see a complete overview of the current status on the
[Compatibility chart](https://github.com/go-kivik/kivik/blob/master/doc/COMPATIBILITY.md)

## Why the name "Kivik"?

[Kivik](http://www.ikea.com/us/en/catalog/categories/series/18329/) is a line
of sofas (couches) from IKEA. And in the spirit of IKEA, and build-your-own
furniture, Kivik aims to allow you to "build your own" CouchDB client, server,
and proxy applications.

## What license is Kivik released under?

This software is released under the terms of the Apache 2.0 license. See
LICENCE.md, or read the [full license](http://www.apache.org/licenses/LICENSE-2.0).

## What projects currently use Kivik?

If your project uses Kivik, and you'd like to be added to this list, create an
issue or submit a pull request.

- [Cayley](https://github.com/cayleygraph/cayley) is an open-source graph
  database. It uses Kivik for the CouchDB and PouchDB storage backends.
