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

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"
	"github.com/knative/eventing-sources/pkg/kncloudevents"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"

	// Imports the Google Cloud Pub/Sub client package.
	"cloud.google.com/go/pubsub"
	"golang.org/x/net/context"
)

const (
	eventType = "google.pubsub.topic.publish"
	// If in the PubSub message attributes this header is set, use
	// it as the Cloud Event type so as to preserve types that flow
	// through the Receive Adapter.
	eventTypeOverrideKey = "ce-type"
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

	ceClient client.Client
}

func (a *Adapter) Start(ctx context.Context) error {
	a.source = fmt.Sprintf("//pubsub.googleapis.com/%s/topics/%s", a.ProjectID, a.TopicID)

	var err error
	// Make the client to pubsub
	if a.client, err = pubsub.NewClient(ctx, a.ProjectID); err != nil {
		return err
	}

	if a.ceClient == nil {
		if a.ceClient, err = kncloudevents.NewDefaultClient(a.SinkURI); err != nil {
			return fmt.Errorf("failed to create cloudevent client: %s", err.Error())
		}
	}

	// Set the subscription from the client
	a.subscription = a.client.Subscription(a.SubscriptionID)

	// Using that subscription, start receiving messages.
	// Note: subscription.Receive is a blocking call.
	return a.subscription.Receive(ctx, func(ctx context.Context, m *pubsub.Message) {
		a.receiveMessage(ctx, &PubSubMessageWrapper{M: m})
	})
}

func (a *Adapter) receiveMessage(ctx context.Context, m PubSubMessage) {
	logger := logging.FromContext(ctx).With(zap.Any("eventID", m.ID()), zap.Any("sink", a.SinkURI))

	logger.Debugw("Received message", zap.Any("messageData", m.Data()))

	err := a.postMessage(ctx, logger, m)
	if err != nil {
		logger.Infof("Failed to post message: %s", err)
		m.Nack()
	} else {
		logger.Debug("Message sent successfully")
		m.Ack()
	}
}
func (a *Adapter) postMessage(ctx context.Context, logger *zap.SugaredLogger, m PubSubMessage) error {
	// TODO: this will break when the upstream sender updates cloudevents versions.
	// The correct thing to do would be to convert the message to a cloudevent if it is one.
	et := eventType
	if override, ok := m.Message().Attributes[eventTypeOverrideKey]; ok {
		et = override
		logger.Infof("overriding the cloud event type with %q", et)
	}

	event := cloudevents.Event{
		Context: cloudevents.EventContextV02{
			Type:        et,
			ID:          m.ID(),
			Time:        &types.Timestamp{Time: m.PublishTime()},
			Source:      *types.ParseURLRef(a.source),
			ContentType: cloudevents.StringOfApplicationJSON(),
		}.AsV02(),
		Data: m.Message(),
	}

	_, err := a.ceClient.Send(ctx, event)
	return err // err could be nil or an error
}
