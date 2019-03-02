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

package bitbucket

import (
	"context"
	"fmt"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"

	"log"
	"net/http"

	sourcesv1alpha1 "github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"

	webhooks "gopkg.in/go-playground/webhooks.v3"
	bb "gopkg.in/go-playground/webhooks.v3/bitbucket"
)

const (
	BBRequestUUID = "Request-UUID"
	BBEventKey    = "Event-Key"
)

// Adapter converts incoming BitBucket webhook events to CloudEvents and
// then sends them to the specified Sink.
type Adapter struct {
	Sink   string
	client client.Client
}

// HandleEvent is invoked whenever an event comes in from BitBucket.
func (ra *Adapter) HandleEvent(payload interface{}, header webhooks.Header) {
	hdr := http.Header(header)
	err := ra.handleEvent(payload, hdr)
	if err != nil {
		log.Printf("unexpected error handling BitBucket event: %s", err)
	}
}

func (ra *Adapter) handleEvent(payload interface{}, hdr http.Header) error {
	if ra.client == nil {
		var err error
		if ra.client, err = client.NewHTTPClient(
			client.WithTarget(ra.Sink),
			client.WithHTTPBinaryEncoding(),
			client.WithUUIDs(),
			client.WithTimeNow(),
		); err != nil {
			return err
		}
	}

	bitBucketEventType := hdr.Get("X-" + BBEventKey)
	eventID := hdr.Get("X-" + BBRequestUUID)
	extensions := map[string]interface{}{
		BBEventKey:    bitBucketEventType,
		BBRequestUUID: eventID,
	}

	log.Printf("Handling %s", bitBucketEventType)

	cloudEventType := fmt.Sprintf("%s.%s", sourcesv1alpha1.BitBucketSourceEventPrefix, bitBucketEventType)
	source, err := sourceFromBitBucketEvent(bb.Event(bitBucketEventType), payload)
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
	return ra.client.Send(context.TODO(), event)
}

func sourceFromBitBucketEvent(bitBucketEvent bb.Event, payload interface{}) (*types.URLRef, error) {
	var url string
	switch bitBucketEvent {
	case bb.RepoPushEvent:
		p := payload.(bb.RepoPushPayload)
		// Assuming there is at least 1 change in a push.
		url = p.Push.Changes[0].Links.Commits.Href
	case bb.RepoForkEvent:
		f := payload.(bb.RepoForkPayload)
		url = f.Repository.Links.Self.Href
	case bb.RepoUpdatedEvent:
		u := payload.(bb.RepoUpdatedPayload)
		url = u.Repository.Links.Self.Href
	case bb.RepoCommitCommentCreatedEvent:
		ccc := payload.(bb.RepoCommitCommentCreatedPayload)
		url = ccc.Comment.Links.Self.Href
	case bb.RepoCommitStatusCreatedEvent:
		csc := payload.(bb.RepoCommitStatusCreatedPayload)
		url = csc.CommitStatus.Links.Self.Href
	case bb.RepoCommitStatusUpdatedEvent:
		csu := payload.(bb.RepoCommitStatusUpdatedPayload)
		url = csu.CommitStatus.Links.Self.Href
	case bb.IssueCreatedEvent:
		ic := payload.(bb.IssueCreatedPayload)
		url = ic.Issue.Links.Self.Href
	case bb.IssueUpdatedEvent:
		iu := payload.(bb.IssueUpdatedPayload)
		url = iu.Issue.Links.Self.Href
	case bb.IssueCommentCreatedEvent:
		icc := payload.(bb.IssueCommentCreatedPayload)
		url = icc.Issue.Links.Self.Href
	case bb.PullRequestCreatedEvent:
		prc := payload.(bb.PullRequestCreatedPayload)
		url = prc.PullRequest.Links.Self.Href
	case bb.PullRequestUpdatedEvent:
		pru := payload.(bb.PullRequestUpdatedPayload)
		url = pru.PullRequest.Links.Self.Href
	case bb.PullRequestApprovedEvent:
		pra := payload.(bb.PullRequestApprovedPayload)
		url = pra.PullRequest.Links.Self.Href
	case bb.PullRequestUnapprovedEvent:
		pru := payload.(bb.PullRequestUnapprovedPayload)
		url = pru.PullRequest.Links.Self.Href
	case bb.PullRequestMergedEvent:
		prm := payload.(bb.PullRequestMergedPayload)
		url = prm.PullRequest.Links.Self.Href
	case bb.PullRequestDeclinedEvent:
		prd := payload.(bb.PullRequestDeclinedPayload)
		url = prd.PullRequest.Links.Self.Href
	case bb.PullRequestCommentCreatedEvent:
		prcc := payload.(bb.PullRequestCommentCreatedPayload)
		url = prcc.PullRequest.Links.Self.Href
	case bb.PullRequestCommentUpdatedEvent:
		prcu := payload.(bb.PullRequestCommentUpdatedPayload)
		url = prcu.PullRequest.Links.Self.Href
	case bb.PullRequestCommentDeletedEvent:
		prcd := payload.(bb.PullRequestCommentDeletedPayload)
		url = prcd.PullRequest.Links.Self.Href
	}
	if url != "" {
		source := types.ParseURLRef(url)
		if source != nil {
			return source, nil
		}
	}

	return nil, fmt.Errorf("no source found in bitbucket event")
}
