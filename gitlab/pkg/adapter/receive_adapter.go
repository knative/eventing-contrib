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
	"net/http"

	"go.uber.org/zap"
	"knative.dev/eventing/pkg/adapter"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/source"

	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/google/uuid"
	webhooks "gopkg.in/go-playground/webhooks.v3"
	"gopkg.in/go-playground/webhooks.v3/gitlab"
)

const (
	glHeaderToken = "X-Gitlab-Token"
	glHeaderEvent = "X-Gitlab-Event"
)

type envConfig struct {
	adapter.EnvConfig

	// Environment variable containing Gitlab secret token
	EnvSecret string `envconfig:"GITLAB_SECRET_TOKEN" required:"true"`
	// Port to listen incoming connections
	Port string `envconfig:"PORT" default:"8080"`
}

// gitLabReceiveAdapter converts incoming GitLab webhook events to
// CloudEvents and then sends them to the specified Sink
type gitLabReceiveAdapter struct {
	logger      *zap.SugaredLogger
	client      cloudevents.Client
	secretToken string
	port        string
}

func NewEnvConfig() adapter.EnvConfigAccessor {
	return &envConfig{}
}

func NewAdapter(ctx context.Context, processed adapter.EnvConfigAccessor, ceClient cloudevents.Client, reporter source.StatsReporter) adapter.Adapter {
	logger := logging.FromContext(ctx)
	env := processed.(*envConfig)

	a := &gitLabReceiveAdapter{
		logger:      logger,
		client:      ceClient,
		port:        env.Port,
		secretToken: env.EnvSecret,
	}

	return a
}

func (ra *gitLabReceiveAdapter) Start(stopCh <-chan struct{}) error {
	hook := gitlab.New(&gitlab.Config{Secret: ra.secretToken})
	hook.RegisterEvents(ra.handleEvent,
		gitlab.PushEvents,
		gitlab.TagEvents,
		gitlab.IssuesEvents,
		gitlab.ConfidentialIssuesEvents,
		gitlab.CommentEvents,
		gitlab.MergeRequestEvents,
		gitlab.WikiPageEvents,
		gitlab.PipelineEvents,
		gitlab.BuildEvents)

	addr := fmt.Sprintf(":%s", ra.port)
	return webhooks.Run(hook, addr, "/")
}

func (ra *gitLabReceiveAdapter) handleEvent(payload interface{}, header webhooks.Header) {
	hdr := http.Header(header)
	gitLabHookToken := hdr.Get(glHeaderToken)
	if gitLabHookToken == "" {
		ra.logger.Errorf("%q header is not set", glHeaderToken)
		return
	}
	if gitLabHookToken != ra.secretToken {
		ra.logger.Errorf("event token doesn't match secret")
		return
	}
	gitLabEventType := hdr.Get(glHeaderEvent)
	if gitLabEventType == "" {
		ra.logger.Errorf("%q header is not set", glHeaderEvent)
		return
	}
	extensions := map[string]interface{}{
		glHeaderEvent: gitLabEventType,
	}

	ra.logger.Infof("Handling %s\n", gitLabEventType)

	uuid, err := uuid.NewRandom()
	if err != nil {
		ra.logger.Errorf("can't generate event ID: %s", err)
		return
	}
	eventID := uuid.String()

	cloudEventType := fmt.Sprintf("%s.%s", "dev.knative.sources.gitlabsource", gitLabEventType)
	source := sourceFromGitLabEvent(gitlab.Event(gitLabEventType), payload)

	err = ra.postMessage(payload, source, cloudEventType, eventID, extensions)
	if err != nil {
		ra.logger.Errorf("cloudevent post message error: %s", err)
		return
	}
}

func (ra *gitLabReceiveAdapter) postMessage(payload interface{}, source, eventType, eventID string,
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
