/*
Copyright 2019 The Knative Authors

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

	gitlab "github.com/knative/eventing-sources/contrib/gitlab/pkg/adapter"
	gl "gopkg.in/go-playground/webhooks.v5/gitlab"
)

const (
	// Environment variable containing the HTTP port
	envPort = "PORT"

	// Environment variable containing GitLab secret token
	envSecret = "GITLAB_SECRET_TOKEN"
)

var validEvents = []gl.Event{
	gl.PushEvents,
	gl.TagEvents,
	gl.IssuesEvents,
	gl.ConfidentialIssuesEvents,
	gl.CommentEvents,
	gl.MergeRequestEvents,
	gl.WikiPageEvents,
	gl.PipelineEvents,
	gl.BuildEvents,
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

	log.Printf("Sink is: %q", *sink)

	ra, err := gitlab.New(*sink)
	if err != nil {
		log.Fatalf("Failed to create gitlab adapter: %s", err.Error())
	}

	hook, _ := gl.New(gl.Options.Secret(secretToken))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		event, err := hook.Parse(r, validEvents...)
		if err != nil {
			if err == gl.ErrEventNotFound {
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
