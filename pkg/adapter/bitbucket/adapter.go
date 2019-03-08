/*
Copyright 2019 The Knative Authors

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
	"log"
	"net/http"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"
	sourcesv1alpha1 "github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	"gopkg.in/go-playground/webhooks.v3"
	bb "gopkg.in/go-playground/webhooks.v3/bitbucket"
)

const (
	bbRequestUUID = "Request-UUID"
	bbEventKey    = "Event-Key"
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

	bitBucketEventType := hdr.Get("X-" + bbEventKey)
	eventID := hdr.Get("X-" + bbRequestUUID)
	extensions := map[string]interface{}{
		bbEventKey:    bitBucketEventType,
		bbRequestUUID: eventID,
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
	// There are many choices to extract the source from. Herein, we try to extract the most informative one.
	// TODO check that these are actually the best places to extract the source from.
	var url string
	switch bitBucketEvent {
	case bb.RepoPushEvent:
		if p, ok := payload.(bb.RepoPushPayload); ok {
			url = p.Repository.Links.Self.Href
		}
	case bb.RepoForkEvent:
		if f, ok := payload.(bb.RepoForkPayload); ok {
			url = f.Repository.Links.Self.Href
		}
	case bb.RepoUpdatedEvent:
		if u, ok := payload.(bb.RepoUpdatedPayload); ok {
			url = u.Repository.Links.Self.Href
		}
	case bb.RepoCommitCommentCreatedEvent:
		if ccc, ok := payload.(bb.RepoCommitCommentCreatedPayload); ok {
			url = ccc.Comment.Links.Self.Href
		}
	case bb.RepoCommitStatusCreatedEvent:
		if csc, ok := payload.(bb.RepoCommitStatusCreatedPayload); ok {
			url = csc.CommitStatus.Links.Self.Href
		}
	case bb.RepoCommitStatusUpdatedEvent:
		if csu, ok := payload.(bb.RepoCommitStatusUpdatedPayload); ok {
			url = csu.CommitStatus.Links.Self.Href
		}
	case bb.IssueCreatedEvent:
		if ic, ok := payload.(bb.IssueCreatedPayload); ok {
			url = ic.Issue.Links.Self.Href
		}
	case bb.IssueUpdatedEvent:
		if iu, ok := payload.(bb.IssueUpdatedPayload); ok {
			url = iu.Issue.Links.Self.Href
		}
	case bb.IssueCommentCreatedEvent:
		if icc, ok := payload.(bb.IssueCommentCreatedPayload); ok {
			url = icc.Issue.Links.Self.Href
		}
	case bb.PullRequestCreatedEvent:
		if prc, ok := payload.(bb.PullRequestCreatedPayload); ok {
			url = prc.PullRequest.Links.Self.Href
		}
	case bb.PullRequestUpdatedEvent:
		if pru, ok := payload.(bb.PullRequestUpdatedPayload); ok {
			url = pru.PullRequest.Links.Self.Href
		}
	case bb.PullRequestApprovedEvent:
		if pra, ok := payload.(bb.PullRequestApprovedPayload); ok {
			url = pra.PullRequest.Links.Self.Href
		}
	case bb.PullRequestUnapprovedEvent:
		if pru, ok := payload.(bb.PullRequestUnapprovedPayload); ok {
			url = pru.PullRequest.Links.Self.Href
		}
	case bb.PullRequestMergedEvent:
		if prm, ok := payload.(bb.PullRequestMergedPayload); ok {
			url = prm.PullRequest.Links.Self.Href
		}
	case bb.PullRequestDeclinedEvent:
		if prd, ok := payload.(bb.PullRequestDeclinedPayload); ok {
			url = prd.PullRequest.Links.Self.Href
		}
	case bb.PullRequestCommentCreatedEvent:
		if prcc, ok := payload.(bb.PullRequestCommentCreatedPayload); ok {
			url = prcc.PullRequest.Links.Self.Href
		}
	case bb.PullRequestCommentUpdatedEvent:
		if prcu, ok := payload.(bb.PullRequestCommentUpdatedPayload); ok {
			url = prcu.PullRequest.Links.Self.Href
		}
	case bb.PullRequestCommentDeletedEvent:
		if prcd, ok := payload.(bb.PullRequestCommentDeletedPayload); ok {
			url = prcd.PullRequest.Links.Self.Href
		}
	}
	if url != "" {
		source := types.ParseURLRef(url)
		if source != nil {
			return source, nil
		}
	}

	return nil, fmt.Errorf("no source found in bitbucket event %q", bitBucketEvent)
}
