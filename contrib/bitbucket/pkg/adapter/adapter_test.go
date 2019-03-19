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

package adapter

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	cehttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	"github.com/google/go-cmp/cmp"
	"gopkg.in/go-playground/webhooks.v3"
	bb "gopkg.in/go-playground/webhooks.v3/bitbucket"
)

const (
	testSource = "http://bitbucket.com/a/b"
)

// testCase holds a single row of our BitBucketSource table tests.
type testCase struct {
	// name is a descriptive name for this test suitable as a first argument to t.Run().
	name string

	// sink the response from the fake sink.
	sink func(http.ResponseWriter, *http.Request)

	// wantErr is true when we expect the test to return an error.
	wantErr bool

	// wantErrMsg contains the pattern to match the returned error message.
	// Implies wantErr = true.
	wantErrMsg string

	// payload contains the BitBucket event payload.
	payload interface{}

	// eventType is the BitBucket event type.
	eventType bb.Event

	// eventID is the BitBucket eventID.
	eventID string

	// wantEventType is the expected CloudEvent EventType.
	wantCloudEventType string

	// wantCloudEventSource is the expected CloudEvent source.
	wantCloudEventSource string
}

var testCases = []testCase{
	{
		name:       "no source",
		payload:    bb.RepoPushPayload{},
		eventType:  bb.RepoPushEvent,
		wantErrMsg: `no source found in bitbucket event "repo:push"`,
	}, {
		name: "valid repo:push",
		payload: func() interface{} {
			p := bb.RepoPushPayload{}
			p.Repository.Links.Self.Href = testSource
			return p
		}(),
		eventType:            bb.RepoPushEvent,
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid repo:fork",
		payload: func() interface{} {
			p := bb.RepoForkPayload{}
			p.Repository.Links.Self.Href = testSource
			return p
		}(),
		eventType:            bb.RepoForkEvent,
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid repo:updated",
		payload: func() interface{} {
			p := bb.RepoUpdatedPayload{}
			p.Repository.Links.Self.Href = testSource
			return p
		}(),
		eventType:            bb.RepoUpdatedEvent,
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid repo:commit_comment_created",
		payload: func() interface{} {
			p := bb.RepoCommitCommentCreatedPayload{}
			p.Comment.Links.Self.Href = testSource
			return p
		}(),
		eventType:            bb.RepoCommitCommentCreatedEvent,
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid repo:commit_status_created",
		payload: func() interface{} {
			p := bb.RepoCommitStatusCreatedPayload{}
			p.CommitStatus.Links.Self.Href = testSource
			return p
		}(),
		eventType:            bb.RepoCommitStatusCreatedEvent,
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid repo:commit_status_updated",
		payload: func() interface{} {
			p := bb.RepoCommitStatusUpdatedPayload{}
			p.CommitStatus.Links.Self.Href = testSource
			return p
		}(),
		eventType:            bb.RepoCommitStatusUpdatedEvent,
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid issue:created",
		payload: func() interface{} {
			p := bb.IssueCreatedPayload{}
			p.Issue.Links.Self.Href = testSource
			return p
		}(),
		eventType:            bb.IssueCreatedEvent,
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid issue:comment_created",
		payload: func() interface{} {
			p := bb.IssueCommentCreatedPayload{}
			p.Issue.Links.Self.Href = testSource
			return p
		}(),
		eventType:            bb.IssueCommentCreatedEvent,
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid pullrequest:created",
		payload: func() interface{} {
			p := bb.PullRequestCreatedPayload{}
			p.PullRequest.Links.Self.Href = testSource
			return p
		}(),
		eventType:            bb.PullRequestCreatedEvent,
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid pullrequest:updated",
		payload: func() interface{} {
			p := bb.PullRequestUpdatedPayload{}
			p.PullRequest.Links.Self.Href = testSource
			return p
		}(),
		eventType:            bb.PullRequestUpdatedEvent,
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid pullrequest:approved",
		payload: func() interface{} {
			p := bb.PullRequestApprovedPayload{}
			p.PullRequest.Links.Self.Href = testSource
			return p
		}(),
		eventType:            bb.PullRequestApprovedEvent,
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid pullrequest:unapproved",
		payload: func() interface{} {
			p := bb.PullRequestUnapprovedPayload{}
			p.PullRequest.Links.Self.Href = testSource
			return p
		}(),
		eventType:            bb.PullRequestUnapprovedEvent,
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid pullrequest:fulfilled",
		payload: func() interface{} {
			p := bb.PullRequestMergedPayload{}
			p.PullRequest.Links.Self.Href = testSource
			return p
		}(),
		eventType:            bb.PullRequestMergedEvent,
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid pullrequest:rejected",
		payload: func() interface{} {
			p := bb.PullRequestDeclinedPayload{}
			p.PullRequest.Links.Self.Href = testSource
			return p
		}(),
		eventType:            bb.PullRequestDeclinedEvent,
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid pullrequest:comment_created",
		payload: func() interface{} {
			p := bb.PullRequestCommentCreatedPayload{}
			p.PullRequest.Links.Self.Href = testSource
			return p
		}(),
		eventType:            bb.PullRequestCommentCreatedEvent,
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid pullrequest:comment_updated",
		payload: func() interface{} {
			p := bb.PullRequestCommentUpdatedPayload{}
			p.PullRequest.Links.Self.Href = testSource
			return p
		}(),
		eventType:            bb.PullRequestCommentUpdatedEvent,
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid pullrequest:comment_deleted",
		payload: func() interface{} {
			p := bb.PullRequestCommentDeletedPayload{}
			p.PullRequest.Links.Self.Href = testSource
			return p
		}(),
		eventType:            bb.PullRequestCommentDeletedEvent,
		wantCloudEventSource: testSource,
		wantErr:              false,
	},
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

		ra := Adapter{
			Sink: sinkServer.URL,
		}
		t.Run(tc.name, tc.runner(t, ra))
	}
}

// runner returns a testing func that can be passed to t.Run.
func (tc *testCase) runner(t *testing.T, ra Adapter) func(t *testing.T) {
	return func(t *testing.T) {
		if tc.eventType == "" {
			t.Fatal("eventType is required for table tests")
		}
		hdr := http.Header{}
		hdr.Set("X-"+bbEventKey, string(tc.eventType))
		hdr.Set("X-"+bbRequestUUID, tc.eventID)
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
	eventType := "repo:push"

	expectedRequest := requestValidation{
		Headers: map[string][]string{
			"ce-specversion":  {"0.2"},
			"ce-id":           {"12345"},
			"ce-time":         {"2019-01-29T09:35:10.69383396-08:00"},
			"ce-type":         {"dev.knative.source.bitbucket.repo:push"},
			"ce-source":       {testSource},
			"ce-request-uuid": {`"12345"`},
			"ce-event-key":    {`"repo:push"`},
			"content-type":    {"application/json"},
		},
		Body: `{"actor":{"type":"","username":"","display_name":"","uuid":"","links":{"self":{"href":""},"html":{"href":""},"avatar":{"href":""}}},"repository":{"type":"","links":{"self":{"href":"http://bitbucket.com/a/b"},"html":{"href":""},"avatar":{"href":""}},"uuid":"","project":{"type":"","project":"","uuid":"","links":{"html":{"href":""},"avatar":{"href":""}},"key":""},"full_name":"","name":"","website":"","owner":{"type":"","username":"","display_name":"","uuid":"","links":{"self":{"href":""},"html":{"href":""},"avatar":{"href":""}}},"scm":"","is_private":false},"push":{"changes":null}}`,
	}

	h := &fakeHandler{
		handler: sinkAccepted, // No tests expect the sink to do anything interesting.
	}
	sinkServer := httptest.NewServer(h)
	defer sinkServer.Close()

	ra := Adapter{
		Sink: sinkServer.URL,
	}

	payload := bb.RepoPushPayload{}
	payload.Repository.Links.Self.Href = testSource
	header := http.Header{}
	header.Set("X-"+bbEventKey, eventType)
	header.Set("X-"+bbRequestUUID, eventID)
	ra.HandleEvent(payload, webhooks.Header(header))

	canonicalizeHeaders(expectedRequest)
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
