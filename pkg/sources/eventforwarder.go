/*
Copyright 2018 The Knative Authors

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

package sources

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/knative/pkg/cloudevents"
	"github.com/pkg/errors"
)

// NewEventForwarder creates a wrapped HTTP client, that can be used
// to safely send events to a sink, correctly handling enveloping the
// event and handling possible error scenarios.
func NewEventForwarder(target string) *EventForwarder {
	return &EventForwarder{target, &http.Client{}}
}

// EventForwarder is a wrapper for an HTTP client in the context of
// sending events safely in a durable manner.
type EventForwarder struct {
	target     string
	httpClient *http.Client
}

// PostEvent sends the event to the defined sink.
func (p *EventForwarder) PostEvent(ctx cloudevents.EventContext, message []byte) error {
	req, err := cloudevents.Binary.NewRequest(p.target, message, ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create http request")
	}
	resp, err := p.httpClient.Do(req)
	if err != nil {
		return errors.Wrap(err, "request failed")
	}

	defer resp.Body.Close()

	// Any non 2xx status-code is considered an error
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := ioutil.ReadAll(resp.Body)
		return fmt.Errorf("request failed: code: %d, body: %s", resp.StatusCode, string(body))
	}

	io.Copy(ioutil.Discard, resp.Body)
	return nil
}
