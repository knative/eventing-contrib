/*
Copyright 2018 The Knative Authors

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

package gcppubsub

import (
	"fmt"
	"github.com/knative/pkg/logging"
	"io/ioutil"
	"net/http"

	"go.uber.org/zap"

	// Imports the Google Cloud Pub/Sub client package.
	"cloud.google.com/go/pubsub"
	"github.com/knative/pkg/cloudevents"
	"golang.org/x/net/context"
)

// Adapter implements the GCP Pub/Sub adapter to deliver Pub/Sub messages from
// a pre-existing topic/subscription to a Sink.
type Adapter struct {
	// ProjectID is the pre-existing gcp project id to use.
	ProjectID string
	// TopicID is the pre-existing gcp pub/sub topic id to use.
	TopicID string
	// SubscriptionID is the pre-existing gcp pub/sub subscription id to use.
	SubscriptionID string
	// SinkURI is the URI messages will be forwarded on to.
	SinkURI string

	source       string
	client       *pubsub.Client
	subscription *pubsub.Subscription
}

func (a *Adapter) Start(ctx context.Context) error {
	a.source = fmt.Sprintf("//pubsub.googleapis.com/%s/topics/%s", a.ProjectID, a.TopicID)

	var err error
	// Make the client to pubsub
	if a.client, err = pubsub.NewClient(ctx, a.ProjectID); err != nil {
		return err
	}

	// Set the subscription from the client
	a.subscription = a.client.Subscription(a.SubscriptionID)

	// Using that subscription, start receiving messages.
	if err := a.subscription.Receive(ctx, a.receiveMessage); err != nil {
		return err
	}
	return nil
}

func (a *Adapter) receiveMessage(ctx context.Context, m *pubsub.Message) {
	logger := logging.FromContext(ctx)
	logger.Debug("Received message", zap.Any("messageData", m.Data))
	err := a.postMessage(ctx, m)
	if err != nil {
		logger.Error("Failed to post message", zap.Error(err))
		m.Nack()
	} else {
		m.Ack()
	}
}
func (a *Adapter) postMessage(ctx context.Context, m *pubsub.Message) error {
	logger := logging.FromContext(ctx)

	event := cloudevents.EventContext{
		CloudEventsVersion: cloudevents.CloudEventsVersion,
		EventType:          "google.pubsub.topic.publish",
		EventID:            m.ID,
		EventTime:          m.PublishTime,
		Source:             a.source,
	}
	req, err := cloudevents.Binary.NewRequest(a.SinkURI, m, event)
	if err != nil {
		logger.Error("Failed to marshal the message.", zap.Error(err), zap.Any("message", m))
		return err
	}

	logger.Debug("Posting message", zap.String("sinkURI", a.SinkURI))
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	logger.Debug("Response", zap.String("status", resp.Status), zap.ByteString("body", body))
	return nil
}
