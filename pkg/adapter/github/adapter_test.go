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
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"

	"github.com/google/go-cmp/cmp"

	cehttp "github.com/cloudevents/sdk-go/pkg/cloudevents/transport/http"
	gh "gopkg.in/go-playground/webhooks.v5/github"
)

const (
	testSubject   = "1234"
	testOwnerRepo = "test-user/test-repo"
)

var (
	testSource = v1alpha1.GitHubEventSource(testOwnerRepo)
)

// testCase holds a single row of our GitHubSource table tests
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
			pl.Repository.ID = id
			return pl
		}(),
		eventType:             "check_suite",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid commit_comment",
		payload: func() interface{} {
			pl := gh.CommitCommentPayload{}
			pl.Comment.HTMLURL = fmt.Sprintf("http://test/%s", testSubject)
			return pl
		}(),
		eventType:             "commit_comment",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid create",
		payload: func() interface{} {
			pl := gh.CreatePayload{}
			pl.RefType = testSubject
			return pl
		}(),
		eventType:             "create",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid delete",
		payload: func() interface{} {
			pl := gh.DeletePayload{}
			pl.RefType = testSubject
			return pl
		}(),
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
		eventType:             "fork",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid gollum",
		payload: func() interface{} {
			pl := gh.GollumPayload{}
			// Leaving the subject as empty.
			return pl
		}(),
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
		eventType:             "integration_installation",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid issue_comment",
		payload: func() interface{} {
			pl := gh.IssueCommentPayload{}
			pl.Comment.HTMLURL = fmt.Sprintf("http://test/%s", testSubject)
			return pl
		}(),
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
		eventType:             "issues",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid label",
		payload: func() interface{} {
			pl := gh.LabelPayload{}
			pl.Label.Name = testSubject
			return pl
		}(),
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
		eventType:             "milestone",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid organization",
		payload: func() interface{} {
			pl := gh.OrganizationPayload{}
			pl.Action = testSubject
			return pl
		}(),
		eventType:             "organization",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid org_block",
		payload: func() interface{} {
			pl := gh.OrgBlockPayload{}
			pl.Action = testSubject
			return pl
		}(),
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
		eventType:             "ping",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid project_card",
		payload: func() interface{} {
			pl := gh.ProjectCardPayload{}
			pl.Action = testSubject
			return pl
		}(),
		eventType:             "project_card",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid project_column",
		payload: func() interface{} {
			pl := gh.ProjectColumnPayload{}
			pl.Action = testSubject
			return pl
		}(),
		eventType:             "project_column",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid project",
		payload: func() interface{} {
			pl := gh.ProjectPayload{}
			pl.Action = testSubject
			return pl
		}(),
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
		eventType:             "public",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid pull_request",
		payload: func() interface{} {
			pl := gh.PullRequestPayload{}
			subject, _ := strconv.ParseInt(testSubject, 10, 64)
			pl.PullRequest.Number = subject
			return pl
		}(),
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
		eventType:             "pull_request_review_comment",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid push",
		payload: func() interface{} {
			pl := gh.PushPayload{}
			pl.Compare = fmt.Sprintf("http://test/%s", testSubject)
			return pl
		}(),
		eventType:             "push",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid release",
		payload: func() interface{} {
			pl := gh.ReleasePayload{}
			pl.Release.TagName = testSubject
			return pl
		}(),
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
		eventType:             "repository",
		wantCloudEventSubject: testSubject,
	}, {
		name: "valid status",
		payload: func() interface{} {
			pl := gh.StatusPayload{}
			pl.Sha = testSubject
			return pl
		}(),
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
		eventType:             "watch",
		wantCloudEventSubject: testSubject,
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

		ra, err := New(sinkServer.URL, testOwnerRepo)
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
		eventID := "12345"
		if tc.eventID != "" {
			eventID = tc.eventID
		}
		hdr := http.Header{}
		hdr.Set("X-GitHub-Event", tc.eventType)
		hdr.Set("X-GitHub-Delivery", eventID)
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

	gotSource := event.Source()
	if testSource != gotSource {
		return nil, fmt.Errorf("want source %s, got %s", testSource, gotSource)
	}

	gotSubject := event.Context.GetSubject()
	if tc.wantCloudEventSubject != "" && tc.wantCloudEventSubject != gotSubject {
		return nil, fmt.Errorf("want subject %s, got %s", tc.wantCloudEventSubject, gotSubject)
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
	eventType := "pull_request"

	expectedRequest := requestValidation{
		Headers: map[string][]string{
			"ce-specversion":     {"0.2"},
			"ce-id":              {"12345"},
			"ce-time":            {"2019-01-29T09:35:10.69383396-08:00"},
			"ce-type":            {"dev.knative.source.github.pull_request"},
			"ce-source":          {testSource},
			"ce-github-delivery": {`"12345"`},
			"ce-github-event":    {`"pull_request"`},
			"ce-subject":         {`"` + testSubject + `"`},

			"content-type": {"application/json"},
		},
		Body: `{"action":"","number":0,"pull_request":{"url":"","id":0,"html_url":"","diff_url":"","patch_url":"","issue_url":"","number":1234,"state":"","locked":false,"title":"","user":{"login":"","id":0,"avatar_url":"","gravatar_id":"","url":"","html_url":"","followers_url":"","following_url":"","gists_url":"","starred_url":"","subscriptions_url":"","organizations_url":"","repos_url":"","events_url":"","received_events_url":"","type":"","site_admin":false},"body":"","created_at":"0001-01-01T00:00:00Z","updated_at":"0001-01-01T00:00:00Z","closed_at":null,"merged_at":null,"merge_commit_sha":null,"assignee":null,"assignees":null,"milestone":null,"commits_url":"","review_comments_url":"","review_comment_url":"","comments_url":"","statuses_url":"","labels":null,"head":{"label":"","ref":"","sha":"","user":{"login":"","id":0,"avatar_url":"","gravatar_id":"","url":"","html_url":"","followers_url":"","following_url":"","gists_url":"","starred_url":"","subscriptions_url":"","organizations_url":"","repos_url":"","events_url":"","received_events_url":"","type":"","site_admin":false},"repo":{"id":0,"name":"","full_name":"","owner":{"login":"","id":0,"avatar_url":"","gravatar_id":"","url":"","html_url":"","followers_url":"","following_url":"","gists_url":"","starred_url":"","subscriptions_url":"","organizations_url":"","repos_url":"","events_url":"","received_events_url":"","type":"","site_admin":false},"private":false,"html_url":"","description":"","fork":false,"url":"","forks_url":"","keys_url":"","collaborators_url":"","teams_url":"","hooks_url":"","issue_events_url":"","events_url":"","assignees_url":"","branches_url":"","tags_url":"","blobs_url":"","git_tags_url":"","git_refs_url":"","trees_url":"","statuses_url":"","languages_url":"","stargazers_url":"","contributors_url":"","subscribers_url":"","subscription_url":"","commits_url":"","git_commits_url":"","comments_url":"","issue_comment_url":"","contents_url":"","compare_url":"","merges_url":"","archive_url":"","downloads_url":"","issues_url":"","pulls_url":"","milestones_url":"","notifications_url":"","labels_url":"","releases_url":"","created_at":"0001-01-01T00:00:00Z","updated_at":"0001-01-01T00:00:00Z","pushed_at":"0001-01-01T00:00:00Z","git_url":"","ssh_url":"","clone_url":"","svn_url":"","homepage":null,"size":0,"stargazers_count":0,"watchers_count":0,"language":null,"has_issues":false,"has_downloads":false,"has_wiki":false,"has_pages":false,"forks_count":0,"mirror_url":null,"open_issues_count":0,"forks":0,"open_issues":0,"watchers":0,"default_branch":""}},"base":{"label":"","ref":"","sha":"","user":{"login":"","id":0,"avatar_url":"","gravatar_id":"","url":"","html_url":"","followers_url":"","following_url":"","gists_url":"","starred_url":"","subscriptions_url":"","organizations_url":"","repos_url":"","events_url":"","received_events_url":"","type":"","site_admin":false},"repo":{"id":0,"name":"","full_name":"","owner":{"login":"","id":0,"avatar_url":"","gravatar_id":"","url":"","html_url":"","followers_url":"","following_url":"","gists_url":"","starred_url":"","subscriptions_url":"","organizations_url":"","repos_url":"","events_url":"","received_events_url":"","type":"","site_admin":false},"private":false,"html_url":"","description":"","fork":false,"url":"","forks_url":"","keys_url":"","collaborators_url":"","teams_url":"","hooks_url":"","issue_events_url":"","events_url":"","assignees_url":"","branches_url":"","tags_url":"","blobs_url":"","git_tags_url":"","git_refs_url":"","trees_url":"","statuses_url":"","languages_url":"","stargazers_url":"","contributors_url":"","subscribers_url":"","subscription_url":"","commits_url":"","git_commits_url":"","comments_url":"","issue_comment_url":"","contents_url":"","compare_url":"","merges_url":"","archive_url":"","downloads_url":"","issues_url":"","pulls_url":"","milestones_url":"","notifications_url":"","labels_url":"","releases_url":"","created_at":"0001-01-01T00:00:00Z","updated_at":"0001-01-01T00:00:00Z","pushed_at":"0001-01-01T00:00:00Z","git_url":"","ssh_url":"","clone_url":"","svn_url":"","homepage":null,"size":0,"stargazers_count":0,"watchers_count":0,"language":null,"has_issues":false,"has_downloads":false,"has_wiki":false,"has_pages":false,"forks_count":0,"mirror_url":null,"open_issues_count":0,"forks":0,"open_issues":0,"watchers":0,"default_branch":""}},"_links":{"self":{"href":""},"html":{"href":""},"issue":{"href":""},"comments":{"href":""},"review_comments":{"href":""},"review_comment":{"href":""},"commits":{"href":""},"statuses":{"href":""}},"merged":false,"mergeable":null,"mergeable_state":"","merged_by":null,"comments":0,"review_comments":0,"commits":0,"additions":0,"deletions":0,"changed_files":0},"label":{"id":0,"url":"","name":"","color":"","default":false},"repository":{"id":0,"name":"","full_name":"","owner":{"login":"","id":0,"avatar_url":"","gravatar_id":"","url":"","html_url":"","followers_url":"","following_url":"","gists_url":"","starred_url":"","subscriptions_url":"","organizations_url":"","repos_url":"","events_url":"","received_events_url":"","type":"","site_admin":false},"private":false,"html_url":"","description":"","fork":false,"url":"","forks_url":"","keys_url":"","collaborators_url":"","teams_url":"","hooks_url":"","issue_events_url":"","events_url":"","assignees_url":"","branches_url":"","tags_url":"","blobs_url":"","git_tags_url":"","git_refs_url":"","trees_url":"","statuses_url":"","languages_url":"","stargazers_url":"","contributors_url":"","subscribers_url":"","subscription_url":"","commits_url":"","git_commits_url":"","comments_url":"","issue_comment_url":"","contents_url":"","compare_url":"","merges_url":"","archive_url":"","downloads_url":"","issues_url":"","pulls_url":"","milestones_url":"","notifications_url":"","labels_url":"","releases_url":"","created_at":"0001-01-01T00:00:00Z","updated_at":"0001-01-01T00:00:00Z","pushed_at":"0001-01-01T00:00:00Z","git_url":"","ssh_url":"","clone_url":"","svn_url":"","homepage":null,"size":0,"stargazers_count":0,"watchers_count":0,"language":null,"has_issues":false,"has_downloads":false,"has_wiki":false,"has_pages":false,"forks_count":0,"mirror_url":null,"open_issues_count":0,"forks":0,"open_issues":0,"watchers":0,"default_branch":""},"sender":{"login":"","id":0,"avatar_url":"","gravatar_id":"","url":"","html_url":"","followers_url":"","following_url":"","gists_url":"","starred_url":"","subscriptions_url":"","organizations_url":"","repos_url":"","events_url":"","received_events_url":"","type":"","site_admin":false},"assignee":null,"requested_reviewer":null,"installation":{"id":0}}`,
	}

	h := &fakeHandler{
		//handler: tc.sink,
		handler: sinkAccepted, // No tests expect the sink to do anything interesting
	}
	sinkServer := httptest.NewServer(h)
	defer sinkServer.Close()

	ra, err := New(sinkServer.URL, testOwnerRepo)
	if err != nil {
		t.Fatal(err)
	}

	payload := gh.PullRequestPayload{}
	subject, _ := strconv.ParseInt(testSubject, 10, 64)
	payload.PullRequest.Number = subject
	header := http.Header{}
	header.Set("X-"+GHHeaderEvent, eventType)
	header.Set("X-"+GHHeaderDelivery, eventID)
	ra.HandleEvent(payload, http.Header(header))

	// TODO(https://github.com/knative/pkg/issues/250): clean this up when there is a shared test client.

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
