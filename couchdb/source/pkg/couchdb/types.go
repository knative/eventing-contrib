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

import "net/url"

// ChangeArguments is the list of arguments accepted by /_changes
type ChangeArguments struct {
	// Start the results from changes after the specified sequence identifier.
	Since string
}

// ChangeResponse represents a Database change
type ChangeResponse struct {
	// Array of changes that were made to the database.
	Results []ChangeResponseResult `json:"results"`

	// Identifier of the last of the sequence identifiers. Currently, this identifier is the same as the sequence identifier of the last item in the results.
	LastSeq string `json:"last_seq"`
}

type ChangeResponseResult struct {
	// Update sequence identifier.
	Seq string `json:"seq"`

	// Document identifier
	ID string `json:"id"`

	// An array that lists the changes that were made to the specific document.
	Changes []ChangeChange `json:"changes"`
}

type ChangeChange struct {
	// Document revision
	Rev string `json:"rev"`
}

func (c *ChangeArguments) Query() url.Values {
	values := url.Values{}
	if c.Since != "" {
		values.Add("since", c.Since)
	}
	return values
}
