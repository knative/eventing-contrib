package couchdb

import (
	"fmt"
	"net/http"

	"github.com/go-kivik/kivik"
)

func missingArg(arg string) error {
	return &kivik.Error{HTTPStatus: http.StatusBadRequest, Err: fmt.Errorf("kivik: %s required", arg)}
}
