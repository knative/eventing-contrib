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
	"os"

	"github.com/knative/eventing-sources/pkg/adapter/github"
	webhooks "gopkg.in/go-playground/webhooks.v3"
	gh "gopkg.in/go-playground/webhooks.v3/github"
)

const (
	// Environment variable containing the HTTP port
	envPort = "PORT"

	// Environment variable containing GitHub secret token
	envSecret = "GITHUB_SECRET_TOKEN"
)

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

	log.Printf("Sink is: %q", *sink)

	ra := &github.Adapter{
		Sink: *sink,
	}

	hook := gh.New(&gh.Config{Secret: secretToken})
	hook.RegisterEvents(ra.HandleEvent,
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
		gh.WatchEvent)

	addr := fmt.Sprintf(":%s", port)
	err := webhooks.Run(hook, addr, "/")
	if err != nil {
		log.Fatalf("Failed to run the webhook")
	}
}
