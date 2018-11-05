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

package main

import (
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	ghclient "github.com/google/go-github/github"
	"github.com/google/uuid"
	sourcesv1alpha1 "github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	"github.com/knative/pkg/cloudevents"
	"gopkg.in/go-playground/webhooks.v3"
	gh "gopkg.in/go-playground/webhooks.v3/github"
)

const (
	// Environment variable containing GitHub secret token
	envSecret = "GITHUB_SECRET_TOKEN"
)

// GitHubHandler holds necessary objects for communicating with
// GitHub.
type GitHubHandler struct {
	client *ghclient.Client
	sink   string
}

// HandlePullRequest is invoked whenever a PullRequest is modified (created, updated, etc.)
func (h *GitHubHandler) HandlePullRequest(payload interface{}, header webhooks.Header) {
	log.Print("Handling Pull Request")

	hdr := http.Header(header)

	pl := payload.(gh.PullRequestPayload)

	source := pl.PullRequest.HTMLURL

	eventType, ok := sourcesv1alpha1.GitHubSourceCloudEventType[hdr.Get("X-GitHub-Event")]
	if !ok {
		eventType = sourcesv1alpha1.GitHubSourceUnsupportedEvent
	}

	eventID := hdr.Get("X-GitHub-Delivery")
	if len(eventID) == 0 {
		if uuid, err := uuid.NewRandom(); err != nil {
			eventID = uuid.String()
		}
	}

	postMessage(h.sink, payload, source, eventType, eventID)
}

func main() {
	sink := flag.String("sink", "", "uri to send events to")

	flag.Parse()

	if sink == nil || *sink == "" {
		log.Fatalf("No sink given")
	}

	secretToken := os.Getenv(envSecret)
	if secretToken == "" {
		log.Fatalf("No secret token given")
	}

	log.Printf("Sink is: %q", sink)

	// Set up the auth for being able to talk to GitHub.
	var tc *http.Client

	client := ghclient.NewClient(tc)

	h := &GitHubHandler{
		client: client,
		sink:   *sink,
	}

	hook := gh.New(&gh.Config{Secret: secretToken})
	// TODO: GitHub has more than just Pull Request Events. This needs to
	// handle them all?
	hook.RegisterEvents(h.HandlePullRequest, gh.PullRequestEvent)

	// TODO(n3wscott): Do we need to configure the PORT?
	err := webhooks.Run(hook, ":8080", "/")
	if err != nil {
		log.Fatalf("Failed to run the webhook")
	}
}

func postMessage(sink string, payload interface{}, source, eventType, eventID string) error {
	ctx := cloudevents.EventContext{
		CloudEventsVersion: cloudevents.CloudEventsVersion,
		EventType:          eventType,
		EventID:            eventID,
		EventTime:          time.Now(),
		Source:             source,
	}
	req, err := cloudevents.Binary.NewRequest(sink, payload, ctx)
	if err != nil {
		log.Printf("Failed to marshal the message: %+v : %s", payload, err)
		return err
	}

	log.Printf("Posting to %q", sink)
	client := &http.Client{}
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
