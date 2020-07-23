/*
Copyright 2020 The Knative Authors

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

package mtadapter

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	adaptertesting "knative.dev/eventing/pkg/adapter/v2/test"
	logtesting "knative.dev/pkg/logging/testing"

	"knative.dev/eventing-contrib/test/lib"
	"knative.dev/eventing-contrib/test/lib/resources"
)

func TestGitHubServer(t *testing.T) {
	logger := logtesting.TestLogger(t)
	ce := adaptertesting.NewTestClient()

	objects := []runtime.Object{resources.NewGitHubSourceV1Alpha1("valid", "path")}
	lister := lib.NewListers(objects).GetGithubSourceLister()
	handler := NewRouter(logger, lister, ce)

	s := httptest.NewServer(handler)
	defer s.Close()

	// Not Found
	resp, err := s.Client().Get(s.URL + "/does/not/exist")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if resp.StatusCode != 404 {
		t.Fatalf("Unexpected status code. Wanted 404, got %d", resp.StatusCode)
	}

	// Registered and in the indexer
	handler.Register("valid", "path", "/valid/path", &fakeHandler{
		handler: sinkAccepted,
	})

	resp, err = s.Client().Get(s.URL + "/valid/path")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("Unexpected status code. Wanted 200, got %d", resp.StatusCode)
	}

	// Registered but not in the indexer
	handler.Register("valid-not-in-cache", "path", "/valid-not-in-cache/path", &fakeHandler{
		handler: sinkAccepted,
	})

	resp, err = s.Client().Get(s.URL + "/valid-not-in-cache/path")

	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if resp.StatusCode != 404 {
		t.Fatalf("Unexpected status code. Wanted 404, got %d", resp.StatusCode)
	}

}

type fakeHandler struct {
	body   []byte
	header http.Header

	handler func(http.ResponseWriter, *http.Request)
}

func (h *fakeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "can not read body", http.StatusBadRequest)
		return
	}
	h.body = body
	h.header = make(map[string][]string)

	for n, v := range r.Header {
		ln := strings.ToLower(n)
		if _, present := unimportantHeaders[ln]; !present {
			h.header[ln] = v
		}
	}

	defer r.Body.Close()
	h.handler(w, r)
}

var (
	// Headers that are added to the response, but we don't want to check in our assertions.
	unimportantHeaders = map[string]struct{}{
		"accept-encoding": {},
		"content-length":  {},
		"user-agent":      {},
		"ce-time":         {},
		"ce-traceparent":  {},
		"traceparent":     {},
		"x-b3-sampled":    {},
		"x-b3-spanid":     {},
		"x-b3-traceid":    {},
	}
)

func sinkAccepted(writer http.ResponseWriter, req *http.Request) {
	writer.WriteHeader(http.StatusOK)
}
