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

package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"gitlab.com/triggermesh/gitlabsource/pkg/ra"
	webhooks "gopkg.in/go-playground/webhooks.v3"
	gitlab "gopkg.in/go-playground/webhooks.v3/gitlab"
)

const (
	// Environment variable containing the HTTP port
	envPort = "PORT"

	// Environment variable containing Gitlab secret token
	envSecret = "GITLAB_SECRET_TOKEN"
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

	ra := &ra.GitLabReceiveAdapter{
		Sink: *sink,
	}

	hook := gitlab.New(&gitlab.Config{Secret: secretToken})
	hook.RegisterEvents(ra.HandleEvent,
		gitlab.PushEvents,
		gitlab.TagEvents,
		gitlab.IssuesEvents,
		gitlab.ConfidentialIssuesEvents,
		gitlab.CommentEvents,
		gitlab.MergeRequestEvents,
		gitlab.WikiPageEvents,
		gitlab.PipelineEvents,
		gitlab.BuildEvents)

	addr := fmt.Sprintf(":%s", port)
	err := webhooks.Run(hook, addr, "/")
	if err != nil {
		log.Fatalf("Failed to run the webhook")
	}
}
