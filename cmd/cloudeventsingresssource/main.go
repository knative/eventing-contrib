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

package main

import (
	"context"
	"log"
	"os"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	cecontext "github.com/cloudevents/sdk-go/pkg/cloudevents/context"
	"github.com/knative/eventing-sources/pkg/kncloudevents"
)

const (
	// Sink to transform CloudEvent data.
	envSinkURI = "SINK_URI"
)

var (
	sink     string
	ceClient client.Client
)

func receive(ctx context.Context, event cloudevents.Event) {
	ec := event.Context.AsV02()
	log.Printf("CloudEvent:\n%s", event)
	log.Printf("[%s] %s %s: ", ec.Time, event.DataContentType(), ec.Source.String())

	//send out
	if _, err := ceClient.Send(ctx, event); err != nil {
		log.Printf("failed to send cloudevent\n%s", err)
	}
}

func main() {
	sink := os.Getenv(envSinkURI)
	if sink == "" {
		log.Fatalf("no sink provided")
	}

	ctx := cecontext.WithTarget(context.Background(), sink)
	ceClient, err := kncloudevents.NewDefaultClient()

	if err != nil {
		log.Fatalf("failed to create client:%s", err)
	}

	log.Fatalf("failed to start receiver:%s", ceClient.StartReceiver(ctx, receive))

	log.Printf("listening on port %d\n", 8080)
	<-ctx.Done()
}
