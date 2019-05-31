/*
Copyright 2019 The TriggerMesh Authors.

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

package ra

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/knative/pkg/cloudevents"
	webhooks "gopkg.in/go-playground/webhooks.v3"
	gitlab "gopkg.in/go-playground/webhooks.v3/gitlab"
)

const (
	GLHeaderEvent = "Gitlab-Event"
)

// GitLabReceiveAdapter converts incoming GitLab webhook events to
// CloudEvents and then sends them to the specified Sink
type GitLabReceiveAdapter struct {
	Sink   string
	Client *http.Client
}

// HandleEvent is invoked whenever an event comes in from GitHub
func (ra *GitLabReceiveAdapter) HandleEvent(payload interface{}, header webhooks.Header) {
	hdr := http.Header(header)
	err := ra.handleEvent(payload, hdr)
	if err != nil {
		log.Printf("unexpected error handling GitLab event: %s", err)
	}
}

func (ra *GitLabReceiveAdapter) handleEvent(payload interface{}, hdr http.Header) error {

	gitLabEventType := hdr.Get("X-" + GLHeaderEvent)
	extensions := map[string]interface{}{
		cloudevents.HeaderExtensionsPrefix + GLHeaderEvent: hdr.Get("X-" + GLHeaderEvent),
	}

	log.Printf("Handling %s", gitLabEventType)

	var eventID string
	if uuid, err := uuid.NewRandom(); err == nil {
		eventID = uuid.String()
	}

	cloudEventType := fmt.Sprintf("%s.%s", "dev.triggermesh.source.gitlab", gitLabEventType)
	source := sourceFromGitLabEvent(gitlab.Event(gitLabEventType), payload)

	return ra.postMessage(payload, source, cloudEventType, eventID, extensions)
}

func (ra *GitLabReceiveAdapter) postMessage(payload interface{}, source, eventType, eventID string,
	extensions map[string]interface{}) error {
	ctx := cloudevents.EventContext{
		CloudEventsVersion: cloudevents.CloudEventsVersion,
		EventType:          eventType,
		EventID:            eventID,
		EventTime:          time.Now(),
		Source:             source,
		Extensions:         extensions,
	}
	req, err := cloudevents.Binary.NewRequest(ra.Sink, payload, ctx)
	if err != nil {
		log.Printf("Failed to marshal the message: %+v : %s", payload, err)
		return err
	}

	log.Printf("Posting to %q", ra.Sink)
	client := ra.Client
	if client == nil {
		client = &http.Client{}
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// TODO: in general, receive adapters may have to be able to retry for error cases.
		log.Printf("response Status: %s", resp.Status)
		body, _ := ioutil.ReadAll(resp.Body)
		log.Printf("response Body: %s", string(body))
	}
	return nil
}

func sourceFromGitLabEvent(gitLabEvent gitlab.Event, payload interface{}) string {
	switch gitLabEvent {
	case gitlab.PushEvents:
		pe := payload.(gitlab.PushEventPayload)
		return pe.Repository.URL
	case gitlab.TagEvents:
		te := payload.(gitlab.TagEventPayload)
		return te.Repository.URL
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
		return be.Repository.URL
	}
	return ""
}
