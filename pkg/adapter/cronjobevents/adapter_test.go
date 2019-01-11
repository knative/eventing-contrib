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

package cronjobevents

import (
	"context"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

func TestPostMessage_ServeHTTP(t *testing.T) {
	testCases := map[string]struct {
		sink    func(http.ResponseWriter, *http.Request)
		reqBody string
		error   bool
	}{
		"happy": {
			sink:    sinkAccepted,
			reqBody: `{"ID":"ABC","EventTime":"0001-01-01T00:00:00Z","Body":"data"}`,
		},
		"rejected": {
			sink:    sinkRejected,
			reqBody: `{"ID":"ABC","EventTime":"0001-01-01T00:00:00Z","Body":"data"}`,
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
				Data:    "data",
				SinkURI: sinkServer.URL,
			}

			m := &Message{
				ID:   "ABC",
				Body: a.Data,
			}
			err := a.postMessage(context.TODO(), zap.S(), m)

			if tc.error && err == nil {
				t.Errorf("expected error, but got %v", err)
			}

			if tc.reqBody != string(h.body) {
				t.Errorf("expected request body %q, but got %q", tc.reqBody, h.body)
			}
		})
	}
}

type fakeHandler struct {
	body []byte

	handler func(http.ResponseWriter, *http.Request)
}

func (h *fakeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
