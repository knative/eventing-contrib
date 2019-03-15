/*
Copyright 2018 The Knative Authors

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

package github

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"
	sourcesv1alpha1 "github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	"github.com/knative/eventing-sources/pkg/kncloudevents"
	webhooks "gopkg.in/go-playground/webhooks.v3"
	gh "gopkg.in/go-playground/webhooks.v3/github"
)

const (
	GHHeaderEvent    = "GitHub-Event"
	GHHeaderDelivery = "GitHub-Delivery"
)

// Adapter converts incoming GitHub webhook events to CloudEvents
type Adapter struct {
	client client.Client
}

// New creates an adapter to convert incoming GitHub webhook events to CloudEvents and
// then sends them to the specified Sink
func New(sinkURI string) (*Adapter, error) {
	a := new(Adapter)
	var err error
	a.client, err = kncloudevents.NewDefaultClient(sinkURI)
	if err != nil {
		return nil, err
	}
	return a, nil
}

// HandleEvent is invoked whenever an event comes in from GitHub
func (a *Adapter) HandleEvent(payload interface{}, header webhooks.Header) {
	hdr := http.Header(header)
	err := a.handleEvent(payload, hdr)
	if err != nil {
		log.Printf("unexpected error handling GitHub event: %s", err)
	}
}

func (a *Adapter) handleEvent(payload interface{}, hdr http.Header) error {
	gitHubEventType := hdr.Get("X-" + GHHeaderEvent)
	eventID := hdr.Get("X-" + GHHeaderDelivery)
	extensions := map[string]interface{}{
		GHHeaderEvent:    gitHubEventType,
		GHHeaderDelivery: eventID,
	}

	log.Printf("Handling %s", gitHubEventType)

	cloudEventType := fmt.Sprintf("%s.%s", sourcesv1alpha1.GitHubSourceEventPrefix, gitHubEventType)
	source, err := sourceFromGitHubEvent(gh.Event(gitHubEventType), payload)
	if err != nil {
		return err
	}

	event := cloudevents.Event{
		Context: cloudevents.EventContextV02{
			ID:         eventID,
			Type:       cloudEventType,
			Source:     *source,
			Extensions: extensions,
		}.AsV02(),
		Data: payload,
	}
	_, err = a.client.Send(context.TODO(), event)
	return err
}

func sourceFromGitHubEvent(gitHubEvent gh.Event, payload interface{}) (*types.URLRef, error) {
	var url string
	switch gitHubEvent {
	case gh.CommitCommentEvent:
		cc := payload.(gh.CommitCommentPayload)
		url = cc.Comment.HTMLURL
	case gh.CreateEvent:
		c := payload.(gh.CreatePayload)
		url = c.Repository.HTMLURL
	case gh.DeleteEvent:
		d := payload.(gh.DeletePayload)
		url = d.Repository.HTMLURL
	case gh.DeploymentEvent:
		d := payload.(gh.DeploymentPayload)
		url = d.Repository.HTMLURL
	case gh.DeploymentStatusEvent:
		d := payload.(gh.DeploymentStatusPayload)
		url = d.Repository.HTMLURL
	case gh.ForkEvent:
		f := payload.(gh.ForkPayload)
		url = f.Forkee.HTMLURL
	case gh.GollumEvent:
		g := payload.(gh.GollumPayload)
		url = g.Repository.HTMLURL
	case gh.InstallationEvent, gh.IntegrationInstallationEvent:
		i := payload.(gh.InstallationPayload)
		url = i.Installation.HTMLURL
	case gh.IssueCommentEvent:
		i := payload.(gh.IssueCommentPayload)
		url = i.Comment.HTMLURL
	case gh.IssuesEvent:
		i := payload.(gh.IssuesPayload)
		url = i.Issue.HTMLURL
	case gh.LabelEvent:
		l := payload.(gh.LabelPayload)
		url = l.Repository.HTMLURL
	case gh.MemberEvent:
		m := payload.(gh.MemberPayload)
		url = m.Repository.HTMLURL
	case gh.MembershipEvent:
		m := payload.(gh.MembershipPayload)
		url = m.Organization.URL
	case gh.MilestoneEvent:
		m := payload.(gh.MilestonePayload)
		url = m.Repository.HTMLURL
	case gh.OrganizationEvent:
		o := payload.(gh.OrganizationPayload)
		url = o.Organization.URL
	case gh.OrgBlockEvent:
		o := payload.(gh.OrgBlockPayload)
		url = o.Organization.URL
	case gh.PageBuildEvent:
		p := payload.(gh.PageBuildPayload)
		url = p.Repository.HTMLURL
	case gh.PingEvent:
		p := payload.(gh.PingPayload)
		url = p.Hook.Config.URL
	case gh.ProjectCardEvent:
		p := payload.(gh.ProjectCardPayload)
		url = p.Repository.HTMLURL
	case gh.ProjectColumnEvent:
		p := payload.(gh.ProjectColumnPayload)
		url = p.Repository.HTMLURL
	case gh.ProjectEvent:
		p := payload.(gh.ProjectPayload)
		url = p.Repository.HTMLURL
	case gh.PublicEvent:
		p := payload.(gh.PublicPayload)
		url = p.Repository.HTMLURL
	case gh.PullRequestEvent:
		p := payload.(gh.PullRequestPayload)
		url = p.PullRequest.HTMLURL
	case gh.PullRequestReviewEvent:
		p := payload.(gh.PullRequestReviewPayload)
		url = p.Review.HTMLURL
	case gh.PullRequestReviewCommentEvent:
		p := payload.(gh.PullRequestReviewCommentPayload)
		url = p.Comment.HTMLURL
	case gh.PushEvent:
		p := payload.(gh.PushPayload)
		url = p.Compare
	case gh.ReleaseEvent:
		r := payload.(gh.ReleasePayload)
		url = r.Release.HTMLURL
	case gh.RepositoryEvent:
		r := payload.(gh.RepositoryPayload)
		url = r.Repository.HTMLURL
	case gh.StatusEvent:
		s := payload.(gh.StatusPayload)
		url = s.Commit.HTMLURL
	case gh.TeamEvent:
		t := payload.(gh.TeamPayload)
		url = t.Organization.URL
	case gh.TeamAddEvent:
		t := payload.(gh.TeamAddPayload)
		url = t.Repository.HTMLURL
	case gh.WatchEvent:
		w := payload.(gh.WatchPayload)
		url = w.Repository.HTMLURL
	}
	if url != "" {
		source := types.ParseURLRef(url)
		if source != nil {
			return source, nil
		}
	}

	return nil, fmt.Errorf("no source found in github event")
}
