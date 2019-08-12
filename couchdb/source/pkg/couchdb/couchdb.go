/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Veroute.on 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package couchdb

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"
)

// Connection represents a Couch DB connection
type Connection interface {
	GetAllDatabases() ([]string, error)
	Changes(database string, args *ChangeArguments) (*ChangeResponse, error)
}

type connection struct {
	url      string
	username string
	password string
	timeout  time.Duration
}

// Connect creates a new couchDB connection
func Connect(fullurl, username, password string, timeout time.Duration) (Connection, error) {
	parsed, err := url.Parse(fullurl)
	if err != nil {
		return nil, err
	}

	connection := &connection{
		url:      parsed.String(), // normalize
		username: username,
		password: password,
	}

	return connection, nil
}

// GetAllDatabases gets all databases in the account
func (c *connection) GetAllDatabases() ([]string, error) {
	raw, err := c.get("_all_dbs", nil)
	if err != nil {
		return nil, err
	}

	var names []string
	err = json.Unmarshal(raw, &names)
	if err != nil {
		return nil, err
	}
	return names, nil
}

// Changes returns a list of changes that were made to documents in the database,
// including insertions, updates, and deletions.
func (c *connection) Changes(database string, args *ChangeArguments) (*ChangeResponse, error) {
	values := url.Values(nil)
	if args != nil {
		values = args.Query()
	}

	raw, err := c.get(database+"/_changes", values)
	if err != nil {
		return nil, err
	}

	fmt.Println(string(raw))

	var changes ChangeResponse
	err = json.Unmarshal(raw, &changes)
	if err != nil {
		return nil, err
	}
	return &changes, nil
}

func (c *connection) get(path string, query url.Values) ([]byte, error) {
	rawurl, err := url.Parse(c.url)
	if err != nil {
		return nil, err
	}
	rawurl.Path = fmt.Sprintf("%s/%s", rawurl.Path, path)
	if query != nil {
		rawurl.RawQuery = query.Encode()
	}

	request, err := http.NewRequest("GET", rawurl.String(), nil)
	if err != nil {
		return nil, err
	}

	request.SetBasicAuth(c.username, c.password)

	client := &http.Client{
		Timeout: c.timeout,
	}

	resp, err := client.Do(request)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return ioutil.ReadAll(resp.Body)
}
