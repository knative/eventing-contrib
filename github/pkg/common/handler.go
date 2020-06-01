/*
Copyright 2020 The Knative Authors

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

package common

import (
	"context"
	"fmt"
	"net/http"
	"time"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	gh "gopkg.in/go-playground/webhooks.v5/github"
	sourcesv1alpha1 "knative.dev/eventing-contrib/github/pkg/apis/sources/v1alpha1"

	"go.uber.org/zap"
)

type Handler struct {
	Logger  *zap.SugaredLogger
	Client  cloudevents.Client
	Hook    *gh.Webhook
	Source  string
	SinkURI string
}

// New creates an adapter to convert incoming GitHub webhook events from a single source
// to CloudEvents and then sends them to the specified Sink
func NewHandler(ceClient cloudevents.Client, sinkURI, source, secretToken string, logger *zap.SugaredLogger) *Handler {
	h := &Handler{
		Logger:  logger,
		Client:  ceClient,
		SinkURI: sinkURI,
		Source:  source,
	}

	h.Hook, _ = gh.New(gh.Options.Secret(secretToken))
	return h
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	payload, err := h.Hook.Parse(r, ValidEvents...)
	if err != nil {
		if err == gh.ErrEventNotFound {
			w.WriteHeader(http.StatusNotFound)
			h.Logger.Info("Event not found")
			return
		}
		w.WriteHeader(http.StatusBadRequest)
		h.Logger.Errorf("Error processing request: %v", err)
		return
	}

	ctx := context.Background()
	if len(h.SinkURI) > 0 {
		ctx = cloudevents.ContextWithTarget(ctx, h.SinkURI)
	}

	err = h.handleEvent(ctx, payload, r.Header)

	if err != nil {
		h.Logger.Errorf("Event handler error: %v", err)
		w.WriteHeader(400)
		w.Write([]byte(err.Error()))
		return
	}
	h.Logger.Infof("Event processed")
	w.WriteHeader(202)
	w.Write([]byte("accepted"))
}

func (h *Handler) handleEvent(ctx context.Context, payload interface{}, hdr http.Header) error {
	gitHubEventType := hdr.Get(GHHeaderEvent)
	if gitHubEventType == "" {
		return fmt.Errorf("%q header is not set", GHHeaderEvent)
	}
	eventID := hdr.Get(GHHeaderDelivery)
	if eventID == "" {
		return fmt.Errorf("%q header is not set", GHHeaderDelivery)
	}

	h.Logger.Infof("Handling %s", gitHubEventType)

	cloudEventType := sourcesv1alpha1.GitHubEventType(gitHubEventType)
	subject := SubjectFromGitHubEvent(gh.Event(gitHubEventType), payload, h.Logger)

	event := cloudevents.NewEvent()
	event.SetID(eventID)
	event.SetType(cloudEventType)
	event.SetSource(h.Source)
	event.SetSubject(subject)
	if err := event.SetData(cloudevents.ApplicationJSON, payload); err != nil {
		return fmt.Errorf("failed to marshal event data: %w", err)
	}

	result := h.Client.Send(ctx, event)
	if !cloudevents.IsACK(result) {
		return result
	}
	return nil
}

// GracefulShutdown gracefully shutdown server
func GracefulShutdown(server *http.Server, logger *zap.SugaredLogger, stopCh <-chan struct{}, done chan<- bool) {
	<-stopCh
	logger.Info("Server is shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	server.SetKeepAlivesEnabled(false)
	if err := server.Shutdown(ctx); err != nil {
		logger.Fatalf("Could not gracefully shutdown the server: %v", err)
	}
	close(done)
}
