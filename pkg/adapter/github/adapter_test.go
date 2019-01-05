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

package github

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/pkg/cloudevents"

	webhooks "gopkg.in/go-playground/webhooks.v3"
	gh "gopkg.in/go-playground/webhooks.v3/github"
)

const (
	testSource = "http://github.com/a/b"
)

// testCase holds a single row of our GitHubSource table tests
type testCase struct {
	// name is a descriptive name for this test suitable as a first argument to t.Run()
	name string

	// wantErr is true when we expect the test to return an error.
	wantErr bool

	// wantErrMsg contains the pattern to match the returned error message.
	// Implies wantErr = true.
	wantErrMsg string

	// payload contains the GitHub event payload
	payload interface{}

	// eventType is the GitHub event type
	eventType string

	// eventID is the GitHub eventID
	eventID string

	// wantEventType is the expected CloudEvent EventType
	wantCloudEventType string

	// wantCloudEventSource is the expected CloudEvent source
	wantCloudEventSource string
}

var testCases = []testCase{
	{
		name:       "no source",
		payload:    gh.PullRequestPayload{},
		eventType:  "pull_request",
		wantErrMsg: `missing required field "Source"`,
	}, {
		name: "valid commit_comment",
		payload: func() interface{} {
			pl := gh.CommitCommentPayload{}
			pl.Comment.HTMLURL = testSource
			return pl
		}(),
		eventType:            "commit_comment",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid create",
		payload: func() interface{} {
			pl := gh.CreatePayload{}
			pl.Repository.HTMLURL = testSource
			return pl
		}(),
		eventType:            "create",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid delete",
		payload: func() interface{} {
			pl := gh.DeletePayload{}
			pl.Repository.HTMLURL = testSource
			return pl
		}(),
		eventType:            "delete",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid deployment",
		payload: func() interface{} {
			pl := gh.DeploymentPayload{}
			pl.Repository.HTMLURL = testSource
			return pl
		}(),
		eventType:            "deployment",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid deployment_status",
		payload: func() interface{} {
			pl := gh.DeploymentStatusPayload{}
			pl.Repository.HTMLURL = testSource
			return pl
		}(),
		eventType:            "deployment_status",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid fork",
		payload: func() interface{} {
			pl := gh.ForkPayload{}
			pl.Forkee.HTMLURL = testSource
			return pl
		}(),
		eventType:            "fork",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid gollum",
		payload: func() interface{} {
			pl := gh.GollumPayload{}
			pl.Repository.HTMLURL = testSource
			return pl
		}(),
		eventType:            "gollum",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid installation",
		payload: func() interface{} {
			pl := gh.InstallationPayload{}
			pl.Installation.HTMLURL = testSource
			return pl
		}(),
		eventType:            "installation",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid integration_installation",
		payload: func() interface{} {
			pl := gh.InstallationPayload{}
			pl.Installation.HTMLURL = testSource
			return pl
		}(),
		eventType:            "integration_installation",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid issue_comment",
		payload: func() interface{} {
			pl := gh.IssueCommentPayload{}
			pl.Comment.HTMLURL = testSource
			return pl
		}(),
		eventType:            "issue_comment",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid issues",
		payload: func() interface{} {
			pl := gh.IssuesPayload{}
			pl.Issue.HTMLURL = testSource
			return pl
		}(),
		eventType:            "issues",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid label",
		payload: func() interface{} {
			pl := gh.LabelPayload{}
			pl.Repository.HTMLURL = testSource
			return pl
		}(),
		eventType:            "label",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid member",
		payload: func() interface{} {
			pl := gh.MemberPayload{}
			pl.Repository.HTMLURL = testSource
			return pl
		}(),
		eventType:            "member",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid membership",
		payload: func() interface{} {
			pl := gh.MembershipPayload{}
			pl.Organization.URL = testSource
			return pl
		}(),
		eventType:            "membership",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid milestone",
		payload: func() interface{} {
			pl := gh.MilestonePayload{}
			pl.Repository.HTMLURL = testSource
			return pl
		}(),
		eventType:            "milestone",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid organization",
		payload: func() interface{} {
			pl := gh.OrganizationPayload{}
			pl.Organization.URL = testSource
			return pl
		}(),
		eventType:            "organization",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid org_block",
		payload: func() interface{} {
			pl := gh.OrgBlockPayload{}
			pl.Organization.URL = testSource
			return pl
		}(),
		eventType:            "org_block",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid page_build",
		payload: func() interface{} {
			pl := gh.PageBuildPayload{}
			pl.Repository.HTMLURL = testSource
			return pl
		}(),
		eventType:            "page_build",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid ping",
		payload: func() interface{} {
			pl := gh.PingPayload{}
			pl.Hook.Config.URL = testSource
			return pl
		}(),
		eventType:            "ping",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid project_card",
		payload: func() interface{} {
			pl := gh.ProjectCardPayload{}
			pl.Repository.HTMLURL = testSource
			return pl
		}(),
		eventType:            "project_card",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid project_column",
		payload: func() interface{} {
			pl := gh.ProjectColumnPayload{}
			pl.Repository.HTMLURL = testSource
			return pl
		}(),
		eventType:            "project_column",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid project",
		payload: func() interface{} {
			pl := gh.ProjectPayload{}
			pl.Repository.HTMLURL = testSource
			return pl
		}(),
		eventType:            "project",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid public",
		payload: func() interface{} {
			pl := gh.PublicPayload{}
			pl.Repository.HTMLURL = testSource
			return pl
		}(),
		eventType:            "public",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid pull_request",
		payload: func() interface{} {
			pl := gh.PullRequestPayload{}
			pl.PullRequest.HTMLURL = testSource
			return pl
		}(),
		eventType:            "pull_request",
		wantCloudEventType:   "dev.knative.source.github.pull_request",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid pull_request_review",
		payload: func() interface{} {
			pl := gh.PullRequestReviewPayload{}
			pl.Review.HTMLURL = testSource
			return pl
		}(),
		eventType:            "pull_request_review",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid pull_request_review_comment",
		payload: func() interface{} {
			pl := gh.PullRequestReviewCommentPayload{}
			pl.Comment.HTMLURL = testSource
			return pl
		}(),
		eventType:            "pull_request_review_comment",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid push",
		payload: func() interface{} {
			pl := gh.PushPayload{}
			pl.Compare = testSource
			return pl
		}(),
		eventType:            "push",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid release",
		payload: func() interface{} {
			pl := gh.ReleasePayload{}
			pl.Release.HTMLURL = testSource
			return pl
		}(),
		eventType:            "release",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid repository",
		payload: func() interface{} {
			pl := gh.RepositoryPayload{}
			pl.Repository.HTMLURL = testSource
			return pl
		}(),
		eventType:            "repository",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid status",
		payload: func() interface{} {
			pl := gh.StatusPayload{}
			pl.Commit.HTMLURL = testSource
			return pl
		}(),
		eventType:            "status",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid team",
		payload: func() interface{} {
			pl := gh.TeamPayload{}
			pl.Organization.URL = testSource
			return pl
		}(),
		eventType:            "team",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid team_add",
		payload: func() interface{} {
			pl := gh.TeamAddPayload{}
			pl.Repository.HTMLURL = testSource
			return pl
		}(),
		eventType:            "team_add",
		wantCloudEventSource: testSource,
		wantErr:              false,
	}, {
		name: "valid watch",
		payload: func() interface{} {
			pl := gh.WatchPayload{}
			pl.Repository.HTMLURL = testSource
			return pl
		}(),
		eventType:            "watch",
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
		client := &http.Client{
			Transport: mockTransport(tc.handleRequest),
		}
		ra := Adapter{
			Sink:   "http://addressable.sink.svc.cluster.local",
			Client: client,
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
		hdr.Set("X-GitHub-Event", tc.eventType)
		hdr.Set("X-GitHub-Delivery", tc.eventID)
		evtErr := ra.handleEvent(tc.payload, hdr)

		if err := tc.verifyErr(evtErr); err != nil {
			t.Error(err)
		}
	}
}

func (tc *testCase) handleRequest(req *http.Request) (*http.Response, error) {
	cloudEvent, err := cloudevents.Binary.FromRequest(nil, req)
	if err != nil {
		return nil, fmt.Errorf("unexpected error decoding cloudevent: %s", err)
	}

	if tc.wantCloudEventType != "" && tc.wantCloudEventType != cloudEvent.EventType {
		return nil, fmt.Errorf("want cloud event type %s, got %s",
			tc.wantCloudEventType, cloudEvent.EventType)
	}

	if tc.wantCloudEventSource != "" && tc.wantCloudEventSource != cloudEvent.Source {
		return nil, fmt.Errorf("want source %s, got %s",
			tc.wantCloudEventSource, cloudEvent.Source)
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

func TestHandleEvent(t *testing.T) {
	eventID := "12345"
	eventType := "pull_request"
	var success bool

	client := &http.Client{
		Transport: mockTransport(func(req *http.Request) (*http.Response, error) {
			cloudEvent, err := cloudevents.Binary.FromRequest(nil, req)
			if err != nil {
				return nil, fmt.Errorf("unexpected error decoding cloudevent: %s", err)
			}
			if eventID != cloudEvent.EventID {
				return nil, fmt.Errorf("want eventID %s, got %s", eventID, cloudEvent.EventID)
			}

			ceHdr := cloudEvent.Extensions[cloudevents.HeaderExtensionsPrefix+GHHeaderDelivery].(string)
			if eventID != ceHdr {
				return nil, fmt.Errorf("%s expected to be %s was %s", GHHeaderDelivery, eventID, ceHdr)
			}

			ceHdr = cloudEvent.Extensions[cloudevents.HeaderExtensionsPrefix+GHHeaderEvent].(string)
			if eventType != ceHdr {
				return nil, fmt.Errorf("%s expected to be %s was %s", GHHeaderEvent, eventType, ceHdr)
			}

			success = true
			return &http.Response{
				StatusCode: 200,
				Body:       ioutil.NopCloser(bytes.NewBufferString("")),
				Header:     make(http.Header),
			}, nil
		}),
	}
	ra := Adapter{
		Sink:   "http://addressable.sink.svc.cluster.local",
		Client: client,
	}
	payload := gh.PullRequestPayload{}
	payload.PullRequest.HTMLURL = testSource
	header := http.Header{}
	header.Set("X-"+GHHeaderEvent, eventType)
	header.Set("X-"+GHHeaderDelivery, eventID)
	ra.HandleEvent(payload, webhooks.Header(header))
	if !success {
		t.Error("did not handle event successfully")
	}
}
