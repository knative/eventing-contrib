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
	"fmt"
	"log"
	"time"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/knative/eventing-sources/pkg/kncloudevents"

	corev1 "k8s.io/api/core/v1"
)

func receive(event cloudevents.Event) {
	metadata := event.Context.AsV02()
	k8sEvent := &corev1.Event{}
	if err := event.DataAs(k8sEvent); err != nil {
		fmt.Printf("got data error: %s\n", err.Error())
	}
	log.Printf("[%s] %s : %q", metadata.Time.Format(time.RFC3339), metadata.Source.String(), k8sEvent.Message)
}

func main() {
	c, err := kncloudevents.NewDefaultClient()
	if err != nil {
		log.Fatalf("failed to create client: %s", err.Error())
	}

	log.Print("listening on port 8080")
	if err := c.StartReceiver(context.Background(), receive); err != nil {
		log.Fatalf("failed to start receiver: %s", err.Error())
	}
}
