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
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/legacy"
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

	// wantRsp contains the pattern to match the returned server response
	wantResp string

	// which status code server should return
	statusCode int

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
		eventType:  gitlab.CommentEvents,
		statusCode: 202,
	}, {
		name: "valid issues",
		payload: func() interface{} {
			pl := gitlab.IssueEventPayload{}
			pl.ObjectAttributes.URL = testSource
			return pl
		}(),
		eventType:  gitlab.IssuesEvents,
		statusCode: 202,
	}, {
		name: "valid push",
		payload: func() interface{} {
			pl := gitlab.PushEventPayload{}
			pl.Project.HTTPURL = testSource
			return pl
		}(),
		eventType:  gitlab.PushEvents,
		statusCode: 202,
	}, {
		name: "valid tag event",
		payload: func() interface{} {
			pl := gitlab.TagEventPayload{}
			pl.Project.HTTPURL = testSource
			return pl
		}(),
		eventType:  gitlab.TagEvents,
		statusCode: 202,
	}, {
		name: "valid confidential issue event",
		payload: func() interface{} {
			pl := gitlab.ConfidentialIssueEventPayload{}
			pl.ObjectAttributes.URL = testSource
			return pl
		}(),
		eventType:  gitlab.ConfidentialIssuesEvents,
		statusCode: 202,
	}, {
		name: "valid merge request event",
		payload: func() interface{} {
			pl := gitlab.MergeRequestEventPayload{}
			pl.ObjectAttributes.URL = testSource
			return pl
		}(),
		eventType:  gitlab.MergeRequestEvents,
		statusCode: 202,
	}, {
		name: "valid wiki page event",
		payload: func() interface{} {
			pl := gitlab.WikiPageEventPayload{}
			pl.ObjectAttributes.URL = testSource
			return pl
		}(),
		eventType:  gitlab.WikiPageEvents,
		statusCode: 202,
	}, {
		name: "valid pipeline event",
		payload: func() interface{} {
			pl := gitlab.PipelineEventPayload{}
			pl.ObjectAttributes.URL = testSource
			return pl
		}(),
		eventType:  gitlab.PipelineEvents,
		statusCode: 202,
	}, {
		name: "valid build event",
		payload: func() interface{} {
			pl := gitlab.BuildEventPayload{}
			pl.Repository.URL = testSource
			return pl
		}(),
		eventType:  gitlab.BuildEvents,
		statusCode: 202,
	}, {
		name:       "invalid nil payload",
		payload:    nil,
		eventType:  gitlab.Event("invalid"),
		wantResp:   "event not registered",
		statusCode: 200,
	}, {
		name:       "invalid empty eventType",
		wantResp:   gitlab.ErrMissingGitLabEventHeader.Error(),
		statusCode: 400,
	},
}

func TestGracefullShutdown(t *testing.T) {
	ce := newTestCeClient()
	ra := newTestAdapter(t, ce)
	stopCh := make(chan struct{}, 1)

	go func(stopCh chan struct{}) {
		defer close(stopCh)
		time.Sleep(time.Second)

	}(stopCh)

	t.Logf("starting webhook server")
	err := ra.Start(stopCh)
	if err != nil {
		t.Error(err)
	}
}

func TestServer(t *testing.T) {
	for _, tc := range testCases {
		ce := newTestCeClient()
		ra := newTestAdapter(t, ce)
		hook, err := gitlab.New(gitlab.Options.Secret(ra.secretToken))
		if err != nil {
			t.Error(err)
		}
		router := ra.newRouter(hook)
		server := httptest.NewServer(router)
		defer server.Close()

		t.Run(tc.name, tc.runner(t, server.URL, ce))
	}
}

func newTestAdapter(t *testing.T, ce *adapterTestCeClient) *gitLabReceiveAdapter {
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

	return NewAdapter(ctx, &env, ce, r).(*gitLabReceiveAdapter)
}

// runner returns a testing func that can be passed to t.Run.
func (tc *testCase) runner(t *testing.T, url string, ceClient *adapterTestCeClient) func(*testing.T) {
	return func(t *testing.T) {
		body, _ := json.Marshal(tc.payload)
		req, err := http.NewRequest("POST", url, bytes.NewReader(body))
		if err != nil {
			t.Error(err)
		}
		req.Header.Set("X-Gitlab-Event", string(tc.eventType))
		req.Header.Set("X-Gitlab-Token", string(secretToken))

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != tc.statusCode {
			t.Errorf("Unexpected status code: %s", resp.Status)
		}

		data, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			t.Error(err)
		}

		if err := tc.validateResponse(t, string(data)); err != nil {
			t.Error(err)
		}
		if err := tc.validateAcceptedPayload(t, ceClient); err != nil {
			t.Error(err)
		}
	}
}

func newTestCeClient() *adapterTestCeClient {
	return &adapterTestCeClient{
		kncetesting.NewTestClient(),
		make(chan struct{}, 1),
	}
}

type adapterTestCeClient struct {
	*kncetesting.TestCloudEventsClient
	stopCh chan struct{}
}

var _ cloudevents.Client = (*adapterTestCeClient)(nil)

type mockReporter struct {
	eventCount int
}

func (r *mockReporter) ReportEventCount(args *source.ReportArgs, responseCode int) error {
	r.eventCount++
	return nil
}

func (tc *testCase) validateAcceptedPayload(t *testing.T, ce *adapterTestCeClient) error {
	if len(ce.Sent()) != 1 {
		return nil
	}

	got := ce.Sent()[0].Data

	data, err := json.Marshal(tc.payload)
	if err != nil {
		return fmt.Errorf("Could not marshal sent payload: %v", err)
	}

	if string(got.([]byte)) != string(data) {
		return fmt.Errorf("Expected %q event to be sent, got %q", data, string(got.([]byte)))
	}
	return nil
}

func (tc *testCase) validateResponse(t *testing.T, message string) error {
	if tc.wantResp != "" && tc.wantResp != message {
		return fmt.Errorf("Expected %q response, got %q", tc.wantResp, message)
	}
	return nil
}
