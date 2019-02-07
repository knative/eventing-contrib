/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package github

import (
	"fmt"
	"log"
	"net/http"

	"github.com/google/uuid"
	sourcesv1alpha1 "github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	"github.com/knative/pkg/cloudevents"
	webhooks "gopkg.in/go-playground/webhooks.v3"
	gh "gopkg.in/go-playground/webhooks.v3/github"
)

const (
	GHHeaderEvent    = "GitHub-Event"
	GHHeaderDelivery = "GitHub-Delivery"
)

// Adapter converts incoming GitHub webhook events to CloudEvents and
// then sends them to the specified Sink
type Adapter struct {
	Sink   string
	client *cloudevents.Client
}

// HandleEvent is invoked whenever an event comes in from GitHub
func (ra *Adapter) HandleEvent(payload interface{}, header webhooks.Header) {
	hdr := http.Header(header)
	err := ra.handleEvent(payload, hdr)
	if err != nil {
		log.Printf("unexpected error handling GitHub event: %s", err)
	}
}

func (ra *Adapter) handleEvent(payload interface{}, hdr http.Header) error {
	if ra.client == nil {
		ra.client = cloudevents.NewClient(ra.Sink, cloudevents.Builder{})
	}

	gitHubEventType := hdr.Get("X-" + GHHeaderEvent)
	eventID := hdr.Get("X-" + GHHeaderDelivery)
	extensions := map[string]interface{}{
		GHHeaderEvent:    hdr.Get("X-" + GHHeaderEvent),
		GHHeaderDelivery: hdr.Get("X-" + GHHeaderDelivery),
	}

	log.Printf("Handling %s", gitHubEventType)

	if len(eventID) == 0 {
		if uuid, err := uuid.NewRandom(); err == nil {
			eventID = uuid.String()
		}
	}

	cloudEventType := fmt.Sprintf("%s.%s", sourcesv1alpha1.GitHubSourceEventPrefix, gitHubEventType)
	source := sourceFromGitHubEvent(gh.Event(gitHubEventType), payload)

	return ra.client.Send(payload, cloudevents.V01EventContext{
		EventType:  cloudEventType,
		EventID:    eventID,
		Source:     source,
		Extensions: extensions,
	})
}

func sourceFromGitHubEvent(gitHubEvent gh.Event, payload interface{}) string {
	switch gitHubEvent {
	case gh.CommitCommentEvent:
		cc := payload.(gh.CommitCommentPayload)
		return cc.Comment.HTMLURL
	case gh.CreateEvent:
		c := payload.(gh.CreatePayload)
		return c.Repository.HTMLURL
	case gh.DeleteEvent:
		d := payload.(gh.DeletePayload)
		return d.Repository.HTMLURL
	case gh.DeploymentEvent:
		d := payload.(gh.DeploymentPayload)
		return d.Repository.HTMLURL
	case gh.DeploymentStatusEvent:
		d := payload.(gh.DeploymentStatusPayload)
		return d.Repository.HTMLURL
	case gh.ForkEvent:
		f := payload.(gh.ForkPayload)
		return f.Forkee.HTMLURL
	case gh.GollumEvent:
		g := payload.(gh.GollumPayload)
		return g.Repository.HTMLURL
	case gh.InstallationEvent, gh.IntegrationInstallationEvent:
		i := payload.(gh.InstallationPayload)
		return i.Installation.HTMLURL
	case gh.IssueCommentEvent:
		i := payload.(gh.IssueCommentPayload)
		return i.Comment.HTMLURL
	case gh.IssuesEvent:
		i := payload.(gh.IssuesPayload)
		return i.Issue.HTMLURL
	case gh.LabelEvent:
		l := payload.(gh.LabelPayload)
		return l.Repository.HTMLURL
	case gh.MemberEvent:
		m := payload.(gh.MemberPayload)
		return m.Repository.HTMLURL
	case gh.MembershipEvent:
		m := payload.(gh.MembershipPayload)
		return m.Organization.URL
	case gh.MilestoneEvent:
		m := payload.(gh.MilestonePayload)
		return m.Repository.HTMLURL
	case gh.OrganizationEvent:
		o := payload.(gh.OrganizationPayload)
		return o.Organization.URL
	case gh.OrgBlockEvent:
		o := payload.(gh.OrgBlockPayload)
		return o.Organization.URL
	case gh.PageBuildEvent:
		p := payload.(gh.PageBuildPayload)
		return p.Repository.HTMLURL
	case gh.PingEvent:
		p := payload.(gh.PingPayload)
		return p.Hook.Config.URL
	case gh.ProjectCardEvent:
		p := payload.(gh.ProjectCardPayload)
		return p.Repository.HTMLURL
	case gh.ProjectColumnEvent:
		p := payload.(gh.ProjectColumnPayload)
		return p.Repository.HTMLURL
	case gh.ProjectEvent:
		p := payload.(gh.ProjectPayload)
		return p.Repository.HTMLURL
	case gh.PublicEvent:
		p := payload.(gh.PublicPayload)
		return p.Repository.HTMLURL
	case gh.PullRequestEvent:
		p := payload.(gh.PullRequestPayload)
		return p.PullRequest.HTMLURL
	case gh.PullRequestReviewEvent:
		p := payload.(gh.PullRequestReviewPayload)
		return p.Review.HTMLURL
	case gh.PullRequestReviewCommentEvent:
		p := payload.(gh.PullRequestReviewCommentPayload)
		return p.Comment.HTMLURL
	case gh.PushEvent:
		p := payload.(gh.PushPayload)
		return p.Compare
	case gh.ReleaseEvent:
		r := payload.(gh.ReleasePayload)
		return r.Release.HTMLURL
	case gh.RepositoryEvent:
		r := payload.(gh.RepositoryPayload)
		return r.Repository.HTMLURL
	case gh.StatusEvent:
		s := payload.(gh.StatusPayload)
		return s.Commit.HTMLURL
	case gh.TeamEvent:
		t := payload.(gh.TeamPayload)
		return t.Organization.URL
	case gh.TeamAddEvent:
		t := payload.(gh.TeamAddPayload)
		return t.Repository.HTMLURL
	case gh.WatchEvent:
		w := payload.(gh.WatchPayload)
		return w.Repository.HTMLURL
	}
	return ""
}
