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
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/knative/eventing-sources/pkg/adapter/github"
	gh "gopkg.in/go-playground/webhooks.v5/github"
)

const (
	// Environment variable containing the HTTP port
	envPort = "PORT"

	// Environment variable containing GitHub secret token
	envSecret = "GITHUB_SECRET_TOKEN"

	// Environment variable containing information about the origin of the event
	envOwnerRepo = "GITHUB_OWNER_REPO"
)

var validEvents = []gh.Event{
	gh.CheckSuiteEvent,
	gh.CommitCommentEvent,
	gh.CommitCommentEvent,
	gh.CreateEvent,
	gh.DeleteEvent,
	gh.DeploymentEvent,
	gh.DeploymentStatusEvent,
	gh.ForkEvent,
	gh.GollumEvent,
	gh.InstallationEvent,
	gh.IntegrationInstallationEvent,
	gh.IssueCommentEvent,
	gh.IssuesEvent,
	gh.LabelEvent,
	gh.MemberEvent,
	gh.MembershipEvent,
	gh.MilestoneEvent,
	gh.OrganizationEvent,
	gh.OrgBlockEvent,
	gh.PageBuildEvent,
	gh.PingEvent,
	gh.ProjectCardEvent,
	gh.ProjectColumnEvent,
	gh.ProjectEvent,
	gh.PublicEvent,
	gh.PullRequestEvent,
	gh.PullRequestReviewEvent,
	gh.PullRequestReviewCommentEvent,
	gh.PushEvent,
	gh.ReleaseEvent,
	gh.RepositoryEvent,
	gh.StatusEvent,
	gh.TeamEvent,
	gh.TeamAddEvent,
	gh.WatchEvent,
}

func main() {
	sink := flag.String("sink", "", "uri to send events to")

	flag.Parse()

	if sink == nil || *sink == "" {
		log.Fatalf("No sink given")
	}

	port := os.Getenv(envPort)
	if port == "" {
		port = "8080"
	}

	secretToken := os.Getenv(envSecret)
	if secretToken == "" {
		log.Fatalf("No secret token given")
	}

	ownerRepo := os.Getenv(envOwnerRepo)
	if ownerRepo == "" {
		log.Fatalf("No ownerRepo given")
	}

	log.Printf("Sink is: %q, OwnerRepo is: %q", *sink, ownerRepo)

	ra, err := github.New(*sink, ownerRepo)
	if err != nil {
		log.Fatalf("Failed to create github adapter: %s", err.Error())
	}

	hook, _ := gh.New(gh.Options.Secret(secretToken))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		event, err := hook.Parse(r, validEvents...)
		if err != nil {
			if err == gh.ErrEventNotFound {
				w.WriteHeader(http.StatusNotFound)

				log.Print("Event not found")
				return
			}
		}

		ra.HandleEvent(event, r.Header)
	})
	addr := fmt.Sprintf(":%s", port)
	http.ListenAndServe(addr, nil)
}
