/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package gcppubsub

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"cloud.google.com/go/pubsub"
	"go.uber.org/zap"
)

func TestPostMessage_ServeHTTP(t *testing.T) {
	testCases := map[string]struct {
		sink              func(http.ResponseWriter, *http.Request)
		reqBody           string
		attributes        map[string]string
		expectedEventType string
		error             bool
	}{
		"happy": {
			sink:    sinkAccepted,
			reqBody: `{"ID":"ABC","Data":"eyJrZXkiOiJ2YWx1ZSJ9","Attributes":null,"PublishTime":"0001-01-01T00:00:00Z"}`,
		},
		"happyWithCustomEvent": {
			sink:              sinkAccepted,
			attributes:        map[string]string{"ce-type": "foobar"},
			reqBody:           `{"ID":"ABC","Data":"eyJrZXkiOiJ2YWx1ZSJ9","Attributes":{"ce-type":"foobar"},"PublishTime":"0001-01-01T00:00:00Z"}`,
			expectedEventType: "foobar",
		},
		"rejected": {
			sink:    sinkRejected,
			reqBody: `{"ID":"ABC","Data":"eyJrZXkiOiJ2YWx1ZSJ9","Attributes":null,"PublishTime":"0001-01-01T00:00:00Z"}`,
			error:   true,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			h := &fakeHandler{
				handler: tc.sink,
			}
			sinkServer := httptest.NewServer(h)
			defer sinkServer.Close()

			a := &Adapter{
				SinkURI: sinkServer.URL,
				source:  "test",
			}

			data, err := json.Marshal(map[string]string{"key": "value"})
			if err != nil {
				t.Errorf("unexpected error, %v", err)
			}

			m := &PubSubMockMessage{
				MockID: "ABC",
				M: &pubsub.Message{
					ID:         "ABC",
					Data:       data,
					Attributes: tc.attributes,
				},
			}
			err = a.postMessage(context.TODO(), zap.S(), m)

			if tc.error && err == nil {
				t.Errorf("expected error, but got %v", err)
			}

			et := ""
			if eth, ok := h.header["Ce-Eventtype"]; ok {
				if len(eth) > 0 {
					et = eth[0]
				}
			}

			expectedEventType := eventType
			if tc.expectedEventType != "" {
				expectedEventType = tc.expectedEventType
			}

			if et != expectedEventType {
				t.Errorf("Expected eventtype %q, but got %q", tc.expectedEventType, et)
			}
			if tc.reqBody != string(h.body) {
				t.Errorf("expected request body %q, but got %q", tc.reqBody, h.body)
			}
		})
	}
}

func TestReceiveMessage_ServeHTTP(t *testing.T) {
	testCases := map[string]struct {
		sink  func(http.ResponseWriter, *http.Request)
		acked bool
	}{
		"happy": {
			sink:  sinkAccepted,
			acked: true,
		},
		"rejected": {
			sink:  sinkRejected,
			acked: false,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			h := &fakeHandler{
				handler: tc.sink,
			}
			sinkServer := httptest.NewServer(h)
			defer sinkServer.Close()

			a := &Adapter{
				SinkURI: sinkServer.URL,
				source:  "test",
			}

			data, err := json.Marshal(map[string]string{"key": "value"})
			if err != nil {
				t.Errorf("unexpected error, %v", err)
			}

			m := &PubSubMockMessage{
				MockID: "ABC",
				M: &pubsub.Message{
					ID:   "ABC",
					Data: data,
				},
			}
			a.receiveMessage(context.TODO(), m)

			if tc.acked && m.Nacked {
				t.Errorf("expected message to be acked, but was not.")
			}

			if !tc.acked && m.Acked {
				t.Errorf("expected message to be nacked, but was not.")
			}

			if m.Acked == m.Nacked {
				t.Errorf("Message has the same Ack and Nack status: %v", m.Acked)
			}
		})
	}
}

type fakeHandler struct {
	body   []byte
	header http.Header

	handler func(http.ResponseWriter, *http.Request)
}

func (h *fakeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.header = r.Header
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "can not read body", http.StatusBadRequest)
		return
	}
	h.body = body

	defer r.Body.Close()
	h.handler(w, r)
}

func sinkAccepted(writer http.ResponseWriter, req *http.Request) {
	writer.WriteHeader(http.StatusOK)
}

func sinkRejected(writer http.ResponseWriter, _ *http.Request) {
	writer.WriteHeader(http.StatusRequestTimeout)
}
