/*
Copyright 2020 The Knative Authors.

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
	"sync"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/google/uuid"
	"go.uber.org/zap"
	"gopkg.in/go-playground/webhooks.v5/gitlab"
	"knative.dev/eventing/pkg/adapter/v2"
	"knative.dev/pkg/logging"
)

const (
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

// NewEnvConfig function reads env variables defined in envConfig structure and
// returns accessor interface
func NewEnvConfig() adapter.EnvConfigAccessor {
	return &envConfig{}
}

// NewAdapter returns the instance of gitLabReceiveAdapter that implements adapter.Adapter interface
func NewAdapter(ctx context.Context, processed adapter.EnvConfigAccessor, ceClient cloudevents.Client) adapter.Adapter {
	logger := logging.FromContext(ctx)
	env := processed.(*envConfig)

	return &gitLabReceiveAdapter{
		logger:      logger,
		client:      ceClient,
		port:        env.Port,
		secretToken: env.EnvSecret,
	}
}

// Start implements adapter.Adapter
func (ra *gitLabReceiveAdapter) Start(stopCh <-chan struct{}) error {
	hook, err := gitlab.New(gitlab.Options.Secret(ra.secretToken))
	if err != nil {
		return fmt.Errorf("cannot create gitlab hook: %v", err)
	}

	server := &http.Server{
		Addr:    ":" + ra.port,
		Handler: ra.newRouter(hook),
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()
		gracefulShutdown(server, ra.logger, stopCh)
	}()

	ra.logger.Infof("Server is ready to handle requests at %s", server.Addr)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("could not listen on %s: %v", server.Addr, err)
	}

	wg.Wait()
	ra.logger.Infof("Server stopped")
	return nil
}

func gracefulShutdown(server *http.Server, logger *zap.SugaredLogger, stopCh <-chan struct{}) {
	<-stopCh
	logger.Info("Server is shutting down...")

	// Try to graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server.SetKeepAlivesEnabled(false)
	if err := server.Shutdown(ctx); err != nil {
		logger.Fatalf("Could not gracefully shutdown the server: %v", err)
	}
}

func (ra *gitLabReceiveAdapter) newRouter(hook *gitlab.Webhook) *http.ServeMux {
	router := http.NewServeMux()
	router.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		payload, err := hook.Parse(r,
			gitlab.PushEvents,
			gitlab.TagEvents,
			gitlab.IssuesEvents,
			gitlab.ConfidentialIssuesEvents,
			gitlab.CommentEvents,
			gitlab.MergeRequestEvents,
			gitlab.WikiPageEvents,
			gitlab.PipelineEvents,
			gitlab.BuildEvents,
		)
		if err != nil {
			if err == gitlab.ErrEventNotFound {
				w.Write([]byte("event not registered"))
				return
			}
			ra.logger.Errorf("hook parser error: %v", err)
			w.WriteHeader(400)
			w.Write([]byte(err.Error()))
			return
		}
		err = ra.handleEvent(payload, r.Header)
		if err != nil {
			ra.logger.Errorf("event handler error: %v", err)
			w.WriteHeader(400)
			w.Write([]byte(err.Error()))
			return
		}
		ra.logger.Infof("event processed")
		w.WriteHeader(202)
		w.Write([]byte("accepted"))
	})
	return router
}

func (ra *gitLabReceiveAdapter) handleEvent(payload interface{}, header http.Header) error {
	gitLabEventType := header.Get(glHeaderEvent)
	if gitLabEventType == "" {
		return fmt.Errorf("%q header is not set", glHeaderEvent)
	}
	extensions := map[string]interface{}{
		glHeaderEvent: gitLabEventType,
	}

	uuid, err := uuid.NewRandom()
	if err != nil {
		return fmt.Errorf("can't generate event ID: %v", err)
	}
	eventID := uuid.String()

	cloudEventType := fmt.Sprintf("%s.%s", "dev.knative.sources.gitlabsource", gitLabEventType)
	source := sourceFromGitLabEvent(gitlab.Event(gitLabEventType), payload)

	return ra.postMessage(payload, source, cloudEventType, eventID, extensions)
}

func (ra *gitLabReceiveAdapter) postMessage(payload interface{}, source, eventType, eventID string,
	extensions map[string]interface{}) error {
	event := cloudevents.NewEvent(cloudevents.VersionV03)
	event.SetID(eventID)
	event.SetType(eventType)
	event.SetSource(source)
	err := event.SetData(cloudevents.ApplicationJSON, payload)
	if err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	result := ra.client.Send(context.Background(), event)
	if !cloudevents.IsACK(result) {
		return result
	}
	return nil
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
