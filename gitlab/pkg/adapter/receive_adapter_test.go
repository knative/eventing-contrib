/*
Copyright 2020 The Knative Authors.

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
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"
	"gopkg.in/go-playground/webhooks.v5/gitlab"
	"knative.dev/eventing/pkg/adapter"
	kncetesting "knative.dev/eventing/pkg/kncloudevents/testing"
	"knative.dev/pkg/logging"
	pkgtesting "knative.dev/pkg/reconciler/testing"
	"knative.dev/pkg/source"
)

const (
	testSource  = "https://foo/bar/baz"
	secretToken = "gitlabsecret"
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
	eventType gitlab.Event
}

var testCases = []testCase{
	{
		name: "valid comment",
		payload: func() interface{} {
			pl := gitlab.CommentEventPayload{}
			pl.ObjectAttributes.URL = testSource
			return pl
		}(),
		eventType: gitlab.CommentEvents,
	}, {
		name: "valid issues",
		payload: func() interface{} {
			pl := gitlab.IssueEventPayload{}
			pl.ObjectAttributes.URL = testSource
			return pl
		}(),
		eventType: gitlab.IssuesEvents,
	}, {
		name: "valid push",
		payload: func() interface{} {
			pl := gitlab.PushEventPayload{}
			pl.Project.HTTPURL = testSource
			return pl
		}(),
		eventType: gitlab.PushEvents,
	},
}

// TestAllCases runs all the table tests
func TestAllCases(t *testing.T) {
	for _, tc := range testCases {
		ce := newAdapterTestClient()
		env := envConfig{
			EnvConfig: adapter.EnvConfig{
				Namespace: "default",
			},
			EnvSecret: secretToken,
			Port:      "8080",
		}
		r := &mockReporter{}
		ctx, _ := pkgtesting.SetupFakeContext(t)
		logger := zap.NewExample().Sugar()
		ctx = logging.WithLogger(ctx, logger)

		ra := NewAdapter(ctx, &env, ce, r).(*gitLabReceiveAdapter)
		t.Run(tc.name, tc.runner(t, ra))
		validateSent(t, ce, tc.payload)
	}
}

// runner returns a testing func that can be passed to t.Run.
func (tc *testCase) runner(t *testing.T, ra *gitLabReceiveAdapter) func(*testing.T) {
	return func(t *testing.T) {
		if tc.eventType == "" {
			t.Fatal("eventType is required for table tests")
		}

		hdr := http.Header{}
		hdr.Set("X-Gitlab-Event", string(tc.eventType))
		hdr.Set("X-Gitlab-Token", string(secretToken))
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

func newAdapterTestClient() *adapterTestClient {
	return &adapterTestClient{
		kncetesting.NewTestClient(),
		make(chan struct{}, 1),
	}
}

type adapterTestClient struct {
	*kncetesting.TestCloudEventsClient
	stopCh chan struct{}
}

var _ cloudevents.Client = (*adapterTestClient)(nil)

type mockReporter struct {
	eventCount int
}

func (r *mockReporter) ReportEventCount(args *source.ReportArgs, responseCode int) error {
	r.eventCount += 1
	return nil
}

func validateSent(t *testing.T, ce *adapterTestClient, wantPayload interface{}) {
	if got := len(ce.Sent()); got != 1 {
		t.Errorf("Expected 1 event to be sent, got %d", got)
	}

	got := ce.Sent()[0].Data

	data, err := json.Marshal(wantPayload)
	if err != nil {
		t.Errorf("Could not validate sent payload: %v", err)
	}

	if string(got.([]byte)) != string(data) {
		t.Errorf("Expected %+v event to be sent, got %q", data, string(got.([]byte)))
	}
}
