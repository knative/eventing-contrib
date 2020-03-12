/*
Copyright 2020 The TriggerMesh Authors.

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
	"context"
	"fmt"
	"log"
	"net/http"

	"knative.dev/eventing-contrib/pkg/kncloudevents"

	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/google/uuid"
	webhooks "gopkg.in/go-playground/webhooks.v3"
	"gopkg.in/go-playground/webhooks.v3/gitlab"
)

const (
	glHeaderToken = "X-Gitlab-Token"
	glHeaderEvent = "X-Gitlab-Event"
)

// GitLabReceiveAdapter converts incoming GitLab webhook events to
// CloudEvents and then sends them to the specified Sink
type GitLabReceiveAdapter struct {
	client      cloudevents.Client
	secretToken string
}

// New creates an adapter to convert incoming GitLab webhook events to CloudEvents and
// then sends them to the specified Sink
func New(sinkURI, secretToken string) (*GitLabReceiveAdapter, error) {
	client, err := kncloudevents.NewDefaultClient(sinkURI)
	if err != nil {
		return nil, err
	}
	return &GitLabReceiveAdapter{
		client:      client,
		secretToken: secretToken,
	}, nil
}

// HandleEvent is invoked whenever an event comes in from GitLab
func (ra *GitLabReceiveAdapter) HandleEvent(payload interface{}, header webhooks.Header) {
	hdr := http.Header(header)
	err := ra.handleEvent(payload, hdr)
	if err != nil {
		log.Printf("unexpected error handling GitLab event: %s", err)
	}
}

func (ra *GitLabReceiveAdapter) handleEvent(payload interface{}, hdr http.Header) error {
	gitLabHookToken := hdr.Get(glHeaderToken)
	if gitLabHookToken == "" {
		return fmt.Errorf("%q header is not set", glHeaderToken)
	}
	if gitLabHookToken != ra.secretToken {
		return fmt.Errorf("event token doesn't match secret")
	}
	gitLabEventType := hdr.Get(glHeaderEvent)
	if gitLabEventType == "" {
		return fmt.Errorf("%q header is not set", glHeaderEvent)
	}
	extensions := map[string]interface{}{
		glHeaderEvent: gitLabEventType,
	}

	log.Printf("Handling %s\n", gitLabEventType)

	uuid, err := uuid.NewRandom()
	if err != nil {
		return fmt.Errorf("can't generate event ID: %s", err)
	}
	eventID := uuid.String()

	cloudEventType := fmt.Sprintf("%s.%s", "dev.knative.sources.gitlabsource", gitLabEventType)
	source := sourceFromGitLabEvent(gitlab.Event(gitLabEventType), payload)

	return ra.postMessage(payload, source, cloudEventType, eventID, extensions)
}

func (ra *GitLabReceiveAdapter) postMessage(payload interface{}, source, eventType, eventID string,
	extensions map[string]interface{}) error {
	event := cloudevents.NewEvent(cloudevents.VersionV03)
	event.SetID(eventID)
	event.SetType(eventType)
	event.SetSource(source)
	event.SetDataContentType(cloudevents.ApplicationJSON)
	event.SetData(payload)

	_, _, err := ra.client.Send(context.Background(), event)
	return err
}

func sourceFromGitLabEvent(gitLabEvent gitlab.Event, payload interface{}) string {
	switch gitLabEvent {
	case gitlab.PushEvents:
		pe := payload.(gitlab.PushEventPayload)
		return pe.Project.HTTPURL
	case gitlab.TagEvents:
		te := payload.(gitlab.TagEventPayload)
		return te.Project.HTTPURL
	case gitlab.IssuesEvents:
		ie := payload.(gitlab.IssueEventPayload)
		return ie.ObjectAttributes.URL
	case gitlab.ConfidentialIssuesEvents:
		cie := payload.(gitlab.ConfidentialIssueEventPayload)
		return cie.ObjectAttributes.URL
	case gitlab.CommentEvents:
		ce := payload.(gitlab.CommentEventPayload)
		return ce.ObjectAttributes.URL
	case gitlab.MergeRequestEvents:
		mre := payload.(gitlab.MergeRequestEventPayload)
		return mre.ObjectAttributes.URL
	case gitlab.WikiPageEvents:
		wpe := payload.(gitlab.WikiPageEventPayload)
		return wpe.ObjectAttributes.URL
	case gitlab.PipelineEvents:
		pe := payload.(gitlab.PipelineEventPayload)
		return pe.ObjectAttributes.URL
	case gitlab.BuildEvents:
		be := payload.(gitlab.BuildEventPayload)
		return be.Repository.Homepage
	}
	return ""
}
