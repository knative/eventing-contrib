package couchdb

// Version is the current version of this package.
const Version = "2.0.0-prerelease"

const (
	// OptionFullCommit is the option key used to set the `X-Couch-Full-Commit`
	// header in the request when set to true.
	//
	// Example:
	//
	//    db.Put(ctx, "doc_id", doc, kivik.Options{couchdb.OptionFullCommit: true})
	OptionFullCommit = "X-Couch-Full-Commit"

	// OptionIfNoneMatch is an option key to set the If-None-Match header on
	// the request.
	//
	// Example:
	//
	//    row, err := db.Get(ctx, "doc_id", kivik.Options{couchdb.OptionIfNoneMatch: "1-xxx"})
	OptionIfNoneMatch = "If-None-Match"

	// NoMultipartPut instructs the Put() method not to use CouchDB's
	// multipart/related upload capabilities. This only affects PUT requests that
	// also include attachments.
	NoMultipartPut = "kivik:no-multipart-put"

	// NoMultipartGet instructs the Get() method not to use CouchDB's ability to
	// download attachments with the multipart/related media type. This only
	// affects GET requests that request attachments.
	NoMultipartGet = "kivik:no-multipart-get"
)

const (
	typeJSON      = "application/json"
	typeMPRelated = "multipart/related"
)
