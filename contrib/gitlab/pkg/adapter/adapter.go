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

package gitlab

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"
	sourcesv1alpha1 "github.com/knative/eventing-sources/contrib/gitlab/pkg/apis/sources/v1alpha1"
	"github.com/knative/eventing-sources/pkg/kncloudevents"
	gl "gopkg.in/go-playground/webhooks.v5/gitlab"
)

const (
	GLHeaderEvent    = "GitLab-Event"
	GLHeaderDelivery = "GitLab-Delivery"
)

// Adapter converts incoming GitLab webhook events to CloudEvents
type Adapter struct {
	client client.Client
}

// New creates an adapter to convert incoming GitLab webhook events to CloudEvents and
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

// HandleEvent is invoked whenever an event comes in from GitLab
func (a *Adapter) HandleEvent(payload interface{}, header http.Header) {
	hdr := http.Header(header)
	err := a.handleEvent(payload, hdr)
	if err != nil {
		log.Printf("unexpected error handling GitLab event: %s", err)
	}
}

func (a *Adapter) handleEvent(payload interface{}, hdr http.Header) error {
	gitLabEventType := hdr.Get("X-" + GLHeaderEvent)
	eventID := hdr.Get("X-" + GLHeaderDelivery)
	extensions := map[string]interface{}{
		GLHeaderEvent:    gitLabEventType,
		GLHeaderDelivery: eventID,
	}

	log.Printf("Handling %s", gitLabEventType)

	cloudEventType := fmt.Sprintf("%s.%s", sourcesv1alpha1.GitLabSourceEventPrefix, gitLabEventType)
	source, err := sourceFromGitLabEvent(gl.Event(gitLabEventType), payload)
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

func sourceFromGitLabEvent(gitLabEvent gl.Event, payload interface{}) (*types.URLRef, error) {
	var url string
	switch gitLabEvent {
	case gl.PushEvents:
		pe := payload.(gl.PushEventPayload)
		url = pe.Repository.URL
	case gl.TagEvents:
		te := payload.(gl.TagEventPayload)
		url = te.Repository.URL
	case gl.IssuesEvents:
		ie := payload.(gl.IssueEventPayload)
		url = ie.ObjectAttributes.URL
	case gl.ConfidentialIssuesEvents:
		cie := payload.(gl.ConfidentialIssueEventPayload)
		url = cie.ObjectAttributes.URL
	case gl.CommentEvents:
		ce := payload.(gl.CommentEventPayload)
		url = ce.ObjectAttributes.URL
	case gl.MergeRequestEvents:
		mre := payload.(gl.MergeRequestEventPayload)
		url = mre.ObjectAttributes.URL
	case gl.WikiPageEvents:
		wpe := payload.(gl.WikiPageEventPayload)
		url = wpe.ObjectAttributes.URL
	case gl.PipelineEvents:
		pe := payload.(gl.PipelineEventPayload)
		url = pe.ObjectAttributes.URL
	case gl.BuildEvents:
		be := payload.(gl.BuildEventPayload)
		url = be.Repository.URL
	}

	log.Printf("url %s", url)
	//Hack here, since gitlab/githab could use ssh url. it's not typical URL style, like this: git@github.com:knative/eventing-sources.git
	//I guess it could be setting in the gitlab/github, but leave the code here to prevent exception
	if url != "" {
		if strings.HasPrefix(url, "git@") {
			url = strings.Replace(url, ":", "/", -1)
			url = strings.Replace(url, "git@", "https://", -1)
			url = strings.Replace(url, ".git", "", -1)
		}

		source := types.ParseURLRef(url)
		log.Printf("source %s", source)
		if source != nil {
			return source, nil
		}
	}

	return nil, fmt.Errorf("no source found in gitlab event")
}
