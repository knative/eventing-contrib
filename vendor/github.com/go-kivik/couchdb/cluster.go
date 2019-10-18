package couchdb

import (
	"context"
	"net/http"

	"github.com/go-kivik/couchdb/chttp"
)

func (c *client) ClusterStatus(ctx context.Context, opts map[string]interface{}) (string, error) {
	var result struct {
		State string `json:"state"`
	}
	query, err := optionsToParams(opts)
	if err != nil {
		return "", err
	}
	_, err = c.DoJSON(ctx, http.MethodGet, "/_cluster_setup", &chttp.Options{Query: query}, &result)
	return result.State, err
}

func (c *client) ClusterSetup(ctx context.Context, action interface{}) error {
	options := &chttp.Options{
		Body: chttp.EncodeBody(action),
	}
	_, err := c.DoError(ctx, http.MethodPost, "/_cluster_setup", options)
	return err
}
