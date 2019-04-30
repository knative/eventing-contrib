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

package gitlab

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	cehttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	gl "gopkg.in/go-playground/webhooks.v5/gitlab"
)

const (
	testSource = "http://gitlab.com/a/b"
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
	eventType string

	// eventID is the GitLab eventID
	eventID string

	// wantEventType is the expected CloudEvent EventType
	wantCloudEventType string

	// wantCloudEventSource is the expected CloudEvent source
	wantCloudEventSource string
}

var testCases = []testCase{
	{
		name:       "no source",
		payload:    gl.PushEventPayload{},
		eventType:  "Push Hook",
		wantErrMsg: `no source found in gitlab event`,
	},
	{
		name: "valid TagEvents",
		payload: func() interface{} {
			pl := gl.TagEventPayload{}
			pl.Repository.URL = testSource
			return pl
		}(),
		eventType:            "Tag Push Hook",
		wantCloudEventSource: testSource,
		wantErr:              false,
	},
	{
		name: "valid IssuesEvents",
		payload: func() interface{} {
			pl := gl.IssueEventPayload{}
			pl.ObjectAttributes.URL = testSource
			return pl
		}(),
		eventType:            "Issue Hook",
		wantCloudEventSource: testSource,
		wantErr:              false,
	},
	{
		name: "valid ConfidentialIssuesEvents",
		payload: func() interface{} {
			pl := gl.ConfidentialIssueEventPayload{}
			pl.ObjectAttributes.URL = testSource
			return pl
		}(),
		eventType:            "Confidential Issue Hook",
		wantCloudEventSource: testSource,
		wantErr:              false,
	},
	{
		name: "valid CommentEvents",
		payload: func() interface{} {
			pl := gl.CommentEventPayload{}
			pl.ObjectAttributes.URL = testSource
			return pl
		}(),
		eventType:            "Note Hook",
		wantCloudEventSource: testSource,
		wantErr:              false,
	},
	{
		name: "valid MergeRequestEvents",
		payload: func() interface{} {
			pl := gl.MergeRequestEventPayload{}
			pl.ObjectAttributes.URL = testSource
			return pl
		}(),
		eventType:            "Merge Request Hook",
		wantCloudEventSource: testSource,
		wantErr:              false,
	},
	{
		name: "valid WikiPageEvents",
		payload: func() interface{} {
			pl := gl.WikiPageEventPayload{}
			pl.ObjectAttributes.URL = testSource
			return pl
		}(),
		eventType:            "Wiki Page Hook",
		wantCloudEventSource: testSource,
		wantErr:              false,
	},
	{
		name: "valid PipelineEvents",
		payload: func() interface{} {
			pl := gl.PipelineEventPayload{}
			pl.ObjectAttributes.URL = testSource
			return pl
		}(),
		eventType:            "Pipeline Hook",
		wantCloudEventSource: testSource,
		wantErr:              false,
	},
	{
		name: "valid BuildEvents",
		payload: func() interface{} {
			pl := gl.BuildEventPayload{}
			pl.Repository.URL = testSource
			return pl
		}(),
		eventType:            "Build Hook",
		wantCloudEventSource: testSource,
		wantErr:              false,
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
			//handler: tc.sink,
			handler: sinkAccepted, // No tests expect the sink to do anything interesting
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
func (tc *testCase) runner(t *testing.T, ra Adapter) func(t *testing.T) {
	return func(t *testing.T) {
		if tc.eventType == "" {
			t.Fatal("eventType is required for table tests")
		}
		hdr := http.Header{}
		hdr.Set("X-GitLab-Event", tc.eventType)
		hdr.Set("X-GitLab-Delivery", tc.eventID)
		evtErr := ra.handleEvent(tc.payload, hdr)

		if err := tc.verifyErr(evtErr); err != nil {
			t.Error(err)
		}
	}
}

func (tc *testCase) handleRequest(req *http.Request) (*http.Response, error) {

	codec := cehttp.Codec{}

	body, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return nil, err
	}
	msg := &cehttp.Message{
		Header: req.Header,
		Body:   body,
	}

	event, err := codec.Decode(msg)
	if err != nil {
		return nil, fmt.Errorf("unexpected error decoding cloudevent: %s", err)
	}

	if tc.wantCloudEventType != "" && tc.wantCloudEventType != event.Type() {
		return nil, fmt.Errorf("want cloud event type %s, got %s",
			tc.wantCloudEventType, event.Type())
	}

	gotSource := event.Context.AsV02().Source

	if tc.wantCloudEventSource != "" && tc.wantCloudEventSource != gotSource.String() {
		return nil, fmt.Errorf("want source %s, got %s",
			tc.wantCloudEventSource, gotSource.String())
	}

	return &http.Response{
		StatusCode: 200,
		Body:       ioutil.NopCloser(bytes.NewBufferString("")),
		Header:     make(http.Header),
	}, nil
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

// Direct Unit tests

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

func TestHandleEvent(t *testing.T) {
	eventID := "12345"
	eventType := "Push Hook"

	expectedRequest := requestValidation{
		Headers: map[string][]string{
			"ce-specversion":     {"0.2"},
			"ce-id":              {"12345"},
			"ce-time":            {"2019-01-29T09:35:10.69383396-08:00"},
			"ce-type":            {"dev.knative.source.gitlab.Push Hook"},
			"ce-source":          {"http://gitlab.com/a/b"},
			"ce-gitlab-delivery": {`"12345"`},
			"ce-gitlab-event":    {`"Push Hook"`},

			"content-type": {"application/json"},
		},
		Body: `{"object_kind":"","before":"","after":"","ref":"","checkout_sha":"","user_id":0,"user_name":"","user_email":"","user_avatar":"","project_id":0,"Project":{"name":"","description":"","web_url":"","avatar_url":"","git_ssh_url":"","git_http_url":"","namespace":"","visibility_level":0,"path_with_namespace":"","default_branch":"","homepage":"","url":"","ssh_url":"","http_url":""},"repository":{"name":"","url":"http://gitlab.com/a/b","description":"","homepage":""},"commits":null,"total_commits_count":0}`,
	}

	h := &fakeHandler{
		//handler: tc.sink,
		handler: sinkAccepted, // No tests expect the sink to do anything interesting
	}
	sinkServer := httptest.NewServer(h)
	defer sinkServer.Close()

	ra, err := New(sinkServer.URL)
	if err != nil {
		t.Fatal(err)
	}

	payload := gl.PushEventPayload{}
	payload.Repository.URL = testSource
	header := http.Header{}
	header.Set("X-"+GLHeaderEvent, eventType)
	header.Set("X-"+GLHeaderDelivery, eventID)
	ra.HandleEvent(payload, header)

	canonicalizeHeaders(expectedRequest)
	// t.Logf(h.header)
	if diff := cmp.Diff(expectedRequest.Headers, h.header); diff != "" {
		t.Errorf("Unexpected difference (-want, +got): %v", diff)
	}

	if diff := cmp.Diff(expectedRequest.Body, string(h.body)); diff != "" {
		t.Errorf("Unexpected difference (-want, +got): %v", diff)
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

func sinkAccepted(writer http.ResponseWriter, req *http.Request) {
	writer.WriteHeader(http.StatusOK)
}

func sinkRejected(writer http.ResponseWriter, _ *http.Request) {
	writer.WriteHeader(http.StatusRequestTimeout)
}

func canonicalizeHeaders(rvs ...requestValidation) {
	// HTTP header names are case-insensitive, so normalize them to lower case for comparison.
	for _, rv := range rvs {
		headers := rv.Headers
		for n, v := range headers {
			delete(headers, n)
			ln := strings.ToLower(n)
			if _, present := unimportantHeaders[ln]; !present {
				headers[ln] = v
			}
		}
	}
}
