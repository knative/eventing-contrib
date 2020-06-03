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

package adapter

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"knative.dev/eventing-contrib/github/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing/pkg/adapter/v2"
	adaptertest "knative.dev/eventing/pkg/adapter/v2/test"
	"knative.dev/pkg/logging"
	pkgtesting "knative.dev/pkg/reconciler/testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/go-cmp/cmp"
	"go.uber.org/zap"
	gh "gopkg.in/go-playground/webhooks.v5/github"
)

const (
	testSubject   = "1234"
	testOwnerRepo = "test-user/test-repo"
	secretToken   = "gitHubsecret"
	eventID       = "12345"
)

var (
	testSource = v1alpha1.GitHubEventSource(testOwnerRepo)
)

// testCase holds a single row of our GitHubSource table tests
type testCase struct {
	// name is a descriptive name for this test suitable as a first argument to t.Run()
	name string

	// payload contains the GitHub event payload
	payload interface{}

	// eventType is the GitHub event type
	eventType string

	// eventID is the GitHub eventID
	eventID string

	// wantEventType is the expected CloudEvent EventType
	wantCloudEventType string

	// wantCloudEventSubject is the expected CloudEvent subject
	wantCloudEventSubject string
}

var testCases = []testCase{
	{
		name: "valid check_suite",
		payload: func() interface{} {
			pl := gh.CheckSuitePayload{}
			id, _ := strconv.ParseInt(testSubject, 10, 64)
			pl.CheckSuite.ID = id
			return pl
		}(),
		eventID:               eventID,
		eventType:             "check_suite",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid commit_comment",
		payload: func() interface{} {
			pl := gh.CommitCommentPayload{}
			pl.Comment.HTMLURL = fmt.Sprintf("http://test/%s", testSubject)
			return pl
		}(),
		eventID:               eventID,
		eventType:             "commit_comment",
		wantCloudEventType:    "dev.knative.source.github.commit_comment",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid create",
		payload: func() interface{} {
			pl := gh.CreatePayload{}
			pl.RefType = testSubject
			return pl
		}(),
		eventID:               eventID,
		eventType:             "create",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid delete",
		payload: func() interface{} {
			pl := gh.DeletePayload{}
			pl.RefType = testSubject
			return pl
		}(),
		eventID:               eventID,
		eventType:             "delete",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid deployment",
		payload: func() interface{} {
			pl := gh.DeploymentPayload{}
			subject, _ := strconv.ParseInt(testSubject, 10, 64)
			pl.Deployment.ID = subject
			return pl
		}(),
		eventID:               eventID,
		eventType:             "deployment",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid deployment_status",
		payload: func() interface{} {
			pl := gh.DeploymentStatusPayload{}
			subject, _ := strconv.ParseInt(testSubject, 10, 64)
			pl.Deployment.ID = subject
			return pl
		}(),
		eventID:               eventID,
		eventType:             "deployment_status",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid fork",
		payload: func() interface{} {
			pl := gh.ForkPayload{}
			subject, _ := strconv.ParseInt(testSubject, 10, 64)
			pl.Forkee.ID = subject
			return pl
		}(),
		eventID:               eventID,
		eventType:             "fork",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid gollum",
		payload: func() interface{} {
			pl := gh.GollumPayload{}
			// Leaving the subject as empty.
			return pl
		}(),
		eventID:               eventID,
		eventType:             "gollum",
		wantCloudEventSubject: "",
	}, {
		name: "valid installation",
		payload: func() interface{} {
			pl := gh.InstallationPayload{}
			subject, _ := strconv.ParseInt(testSubject, 10, 64)
			pl.Installation.ID = subject
			return pl
		}(),
		eventID:               eventID,
		eventType:             "installation",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid integration_installation",
		payload: func() interface{} {
			pl := gh.InstallationPayload{}
			subject, _ := strconv.ParseInt(testSubject, 10, 64)
			pl.Installation.ID = subject
			return pl
		}(),
		eventID:               eventID,
		eventType:             "integration_installation",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid issue_comment",
		payload: func() interface{} {
			pl := gh.IssueCommentPayload{}
			pl.Comment.HTMLURL = fmt.Sprintf("http://test/%s", testSubject)
			return pl
		}(),
		eventID:               eventID,
		eventType:             "issue_comment",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid issues",
		payload: func() interface{} {
			pl := gh.IssuesPayload{}
			subject, _ := strconv.ParseInt(testSubject, 10, 64)
			pl.Issue.Number = subject
			return pl
		}(),
		eventID:               eventID,
		eventType:             "issues",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid label",
		payload: func() interface{} {
			pl := gh.LabelPayload{}
			pl.Label.Name = testSubject
			return pl
		}(),
		eventID:               eventID,
		eventType:             "label",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid member",
		payload: func() interface{} {
			pl := gh.MemberPayload{}
			subject, _ := strconv.ParseInt(testSubject, 10, 64)
			pl.Member.ID = subject
			return pl
		}(),
		eventID:               eventID,
		eventType:             "member",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid membership",
		payload: func() interface{} {
			pl := gh.MembershipPayload{}
			subject, _ := strconv.ParseInt(testSubject, 10, 64)
			pl.Member.ID = subject
			return pl
		}(),
		eventID:               eventID,
		eventType:             "membership",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid milestone",
		payload: func() interface{} {
			pl := gh.MilestonePayload{}
			subject, _ := strconv.ParseInt(testSubject, 10, 64)
			pl.Milestone.Number = subject
			return pl
		}(),
		eventID:               eventID,
		eventType:             "milestone",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid organization",
		payload: func() interface{} {
			pl := gh.OrganizationPayload{}
			pl.Action = testSubject
			return pl
		}(),
		eventID:               eventID,
		eventType:             "organization",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid org_block",
		payload: func() interface{} {
			pl := gh.OrgBlockPayload{}
			pl.Action = testSubject
			return pl
		}(),
		eventID:               eventID,
		eventType:             "org_block",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid page_build",
		payload: func() interface{} {
			pl := gh.PageBuildPayload{}
			subject, _ := strconv.ParseInt(testSubject, 10, 64)
			pl.ID = subject
			return pl
		}(),
		eventID:               eventID,
		eventType:             "page_build",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid ping",
		payload: func() interface{} {
			pl := gh.PingPayload{}
			subject, _ := strconv.Atoi(testSubject)
			pl.HookID = subject
			return pl
		}(),
		eventID:               eventID,
		eventType:             "ping",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid project_card",
		payload: func() interface{} {
			pl := gh.ProjectCardPayload{}
			pl.Action = testSubject
			return pl
		}(),
		eventID:               eventID,
		eventType:             "project_card",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid project_column",
		payload: func() interface{} {
			pl := gh.ProjectColumnPayload{}
			pl.Action = testSubject
			return pl
		}(),
		eventID:               eventID,
		eventType:             "project_column",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid project",
		payload: func() interface{} {
			pl := gh.ProjectPayload{}
			pl.Action = testSubject
			return pl
		}(),
		eventID:               eventID,
		eventType:             "project",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid public",
		payload: func() interface{} {
			pl := gh.PublicPayload{}
			subject, _ := strconv.ParseInt(testSubject, 10, 64)
			pl.Repository.ID = subject
			return pl
		}(),
		eventID:               eventID,
		eventType:             "public",
		wantCloudEventSubject: testSubject,
		wantCloudEventType:    "dev.knative.source.github.public",
	}, {
		name: "valid pull_request",
		payload: func() interface{} {
			pl := gh.PullRequestPayload{}
			subject, _ := strconv.ParseInt(testSubject, 10, 64)
			pl.PullRequest.Number = subject
			return pl
		}(),
		eventID:               eventID,
		eventType:             "pull_request",
		wantCloudEventType:    "dev.knative.source.github.pull_request",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid pull_request_review",
		payload: func() interface{} {
			pl := gh.PullRequestReviewPayload{}
			subject, _ := strconv.ParseInt(testSubject, 10, 64)
			pl.Review.ID = subject
			return pl
		}(),
		eventID:               eventID,
		eventType:             "pull_request_review",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid pull_request_review_comment",
		payload: func() interface{} {
			pl := gh.PullRequestReviewCommentPayload{}
			subject, _ := strconv.ParseInt(testSubject, 10, 64)
			pl.Comment.ID = subject
			return pl
		}(),
		eventID:               eventID,
		eventType:             "pull_request_review_comment",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid push",
		payload: func() interface{} {
			pl := gh.PushPayload{}
			pl.Compare = fmt.Sprintf("http://test/%s", testSubject)
			return pl
		}(),
		eventID:               eventID,
		eventType:             "push",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid release",
		payload: func() interface{} {
			pl := gh.ReleasePayload{}
			pl.Release.TagName = testSubject
			return pl
		}(),
		eventID:               eventID,
		eventType:             "release",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid repository",
		payload: func() interface{} {
			pl := gh.RepositoryPayload{}
			subject, _ := strconv.ParseInt(testSubject, 10, 64)
			pl.Repository.ID = subject
			return pl
		}(),
		eventID:               eventID,
		eventType:             "repository",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid status",
		payload: func() interface{} {
			pl := gh.StatusPayload{}
			pl.Sha = testSubject
			return pl
		}(),
		eventID:               eventID,
		eventType:             "status",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid team",
		payload: func() interface{} {
			pl := gh.TeamPayload{}
			subject, _ := strconv.ParseInt(testSubject, 10, 64)
			pl.Team.ID = subject
			return pl
		}(),
		eventID:               eventID,
		eventType:             "team",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid team_add",
		payload: func() interface{} {
			pl := gh.TeamAddPayload{}
			subject, _ := strconv.ParseInt(testSubject, 10, 64)
			pl.Repository.ID = subject
			return pl
		}(),
		eventID:               eventID,
		eventType:             "team_add",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid watch",
		payload: func() interface{} {
			pl := gh.WatchPayload{}
			subject, _ := strconv.ParseInt(testSubject, 10, 64)
			pl.Repository.ID = subject
			return pl
		}(),
		eventID:               eventID,
		eventType:             "watch",
		wantCloudEventSubject: testSubject,
	},
}

func newTestAdapter(t *testing.T, ce cloudevents.Client) *gitHubAdapter {
	env := envConfig{
		EnvConfig: adapter.EnvConfig{
			Namespace: "default",
		},
		EnvSecret:    secretToken,
		EnvPort:      "12341",
		EnvOwnerRepo: "test.repo",
	}
	ctx, _ := pkgtesting.SetupFakeContext(t)
	logger := zap.NewExample().Sugar()
	ctx = logging.WithLogger(ctx, logger)

	return NewAdapter(ctx, &env, ce).(*gitHubAdapter)
}

func TestGracefulShutdown(t *testing.T) {
	ce := adaptertest.NewTestClient()
	ra := newTestAdapter(t, ce)
	ctx, cancel := context.WithCancel(context.Background())

	go func(cancel context.CancelFunc) {
		defer cancel()
		time.Sleep(time.Second)

	}(cancel)

	t.Logf("starting webhook server")

	err := ra.Start(ctx)
	if err != nil {
		t.Error(err)
	}
}

func TestServer(t *testing.T) {
	for _, tc := range testCases {
		ce := adaptertest.NewTestClient()
		adapter := newTestAdapter(t, ce)
		hook, err := gh.New()
		if err != nil {
			t.Fatal(err)
		}
		router := adapter.newRouter(hook)
		server := httptest.NewServer(router)
		defer server.Close()

		t.Run(tc.name, tc.runner(t, server.URL, ce))
	}
}

// runner returns a testing func that can be passed to t.Run.
func (tc *testCase) runner(t *testing.T, url string, ceClient *adaptertest.TestCloudEventsClient) func(t *testing.T) {
	return func(t *testing.T) {
		if tc.eventType == "" {
			t.Fatal("eventType is required for table tests")
		}
		body, _ := json.Marshal(tc.payload)
		req, err := http.NewRequest("POST", url, bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}

		req.Header.Set(GHHeaderEvent, tc.eventType)
		req.Header.Set(GHHeaderDelivery, eventID)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Error(err)
		}
		defer resp.Body.Close()

		tc.validateAcceptedPayload(t, ceClient)
	}
}

func (tc *testCase) validateAcceptedPayload(t *testing.T, ce *adaptertest.TestCloudEventsClient) {
	t.Helper()
	if len(ce.Sent()) != 1 {
		return
	}
	eventSubject := ce.Sent()[0].Subject()
	if eventSubject != tc.wantCloudEventSubject {
		t.Fatalf("Expected %q event subject to be sent, got %q", tc.wantCloudEventSubject, eventSubject)
	}

	if tc.wantCloudEventType != "" {
		eventType := ce.Sent()[0].Type()
		if eventType != tc.wantCloudEventType {
			t.Fatalf("Expected %q event type to be sent, got %q", tc.wantCloudEventType, eventType)
		}
	}

	eventID := ce.Sent()[0].ID()
	if eventID != tc.eventID {
		t.Fatalf("Expected %q event id to be sent, got %q", tc.eventID, eventID)
	}
	data := ce.Sent()[0].Data()

	var got interface{}
	var want interface{}

	err := json.Unmarshal(data, &got)
	if err != nil {
		t.Fatalf("Could not unmarshal sent data: %v", err)
	}
	payload, err := json.Marshal(tc.payload)
	if err != nil {
		t.Fatalf("Could not marshal sent payload: %v", err)
	}
	err = json.Unmarshal(payload, &want)
	if err != nil {
		t.Fatalf("Could not unmarshal sent payload: %v", err)
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("unexpected event data (-want, +got) = %v", diff)
	}
}
