/*
Copyright 2020 The TriggerMesh Authors.

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

package adapter

import (
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	cehttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	gl "gopkg.in/go-playground/webhooks.v3/gitlab"
)

const (
	testSource = "https://foo/bar/baz"
)

// testCase holds a single row of our GitLabSource table tests
type testCase struct {
	// name is a descriptive name for this test suitable as a first argument to t.Run()
	name string

	// sink the response from the fake sink
	sink func(http.ResponseWriter, *http.Request)

	// wantErr is true when we expect the test to return an error.
	wantErr bool

	// wantErrMsg contains the pattern to match the returned error message.
	// Implies wantErr = true.
	wantErrMsg string

	// payload contains the GitLab event payload
	payload interface{}

	// eventType is the GitLab event type
	eventType gl.Event
}

var testCases = []testCase{
	{
		name: "valid comment",
		payload: func() interface{} {
			pl := gl.CommentEventPayload{}
			pl.ObjectAttributes.URL = testSource
			return pl
		}(),
		eventType: gl.CommentEvents,
	}, {
		name: "valid issues",
		payload: func() interface{} {
			pl := gl.IssueEventPayload{}
			pl.ObjectAttributes.URL = testSource
			return pl
		}(),
		eventType: gl.IssuesEvents,
	}, {
		name: "valid push",
		payload: func() interface{} {
			pl := gl.PushEventPayload{}
			pl.Project.HTTPURL = testSource
			return pl
		}(),
		eventType: gl.PushEvents,
	},
}

// mockTransport is a simple fake HTTP transport
type mockTransport func(req *http.Request) (*http.Response, error)

// RoundTrip implements the required RoundTripper interface for
// mockTransport
func (mt mockTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	return mt(req)
}

// TestAllCases runs all the table tests
func TestAllCases(t *testing.T) {
	for _, tc := range testCases {
		h := &fakeHandler{
			handler: tc.verifyRequest,
		}
		sinkServer := httptest.NewServer(h)
		defer sinkServer.Close()

		ra, err := New(sinkServer.URL)
		if err != nil {
			t.Fatal(err)
		}

		t.Run(tc.name, tc.runner(t, *ra))
	}
}

// runner returns a testing func that can be passed to t.Run.
func (tc *testCase) runner(t *testing.T, ra GitLabReceiveAdapter) func(t *testing.T) {
	return func(t *testing.T) {
		if tc.eventType == "" {
			t.Fatal("eventType is required for table tests")
		}

		hdr := http.Header{}
		hdr.Set("X-Gitlab-Event", string(tc.eventType))
		evtErr := ra.handleEvent(tc.payload, hdr)

		if err := tc.verifyErr(evtErr); err != nil {
			t.Error(err)
		}
	}
}

func (tc *testCase) verifyErr(err error) error {
	wantErr := tc.wantErr || tc.wantErrMsg != ""

	if wantErr && err == nil {
		return errors.New("want error, got nil")
	}

	if !wantErr && err != nil {
		return fmt.Errorf("want no error, got %v", err)
	}

	if err != nil {
		if diff := cmp.Diff(tc.wantErrMsg, err.Error()); diff != "" {
			return fmt.Errorf("incorrect error (-want, +got): %v", diff)
		}
	}
	return nil
}

var (
	// Headers that are added to the response, but we don't want to check in our assertions.
	unimportantHeaders = map[string]struct{}{
		"accept-encoding": {},
		"content-length":  {},
		"user-agent":      {},
		"ce-time":         {},
	}
)

type requestValidation struct {
	Host    string
	Headers http.Header
	Body    string
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

func (tc *testCase) verifyRequest(writer http.ResponseWriter, req *http.Request) {
	codec := cehttp.Codec{}

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
	}
	msg := &cehttp.Message{
		Header: req.Header,
		Body:   body,
	}

	event, err := codec.Decode(context.Background(), msg)
	if err != nil {
		http.Error(writer, "can't decode body", http.StatusBadRequest)
		return
	}

	if tc.eventType != "" && fmt.Sprintf("dev.triggermesh.source.gitlab.%s", tc.eventType) != event.Type() {
		http.Error(writer, "event type is not matching", http.StatusBadRequest)
		return
	}

	if testSource != event.Source() {
		http.Error(writer, "event source is not matching", http.StatusBadRequest)
		return
	}

	writer.WriteHeader(http.StatusOK)
}
