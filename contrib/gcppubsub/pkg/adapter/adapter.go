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

	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/knative/eventing-contrib/pkg/kncloudevents"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"

	// Imports the Google Cloud Pub/Sub client package.
	"cloud.google.com/go/pubsub"
	sourcesv1alpha1 "github.com/knative/eventing-contrib/contrib/gcppubsub/pkg/apis/sources/v1alpha1"
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
	// TransformerURI is the URI messages will be forwarded on to for any transformation
	// before they are sent to SinkURI.
	TransformerURI string

	source       string
	client       *pubsub.Client
	subscription *pubsub.Subscription

	ceClient cloudevents.Client

	transformer       bool
	transformerClient cloudevents.Client
}

func (a *Adapter) Start(ctx context.Context) error {
	a.source = sourcesv1alpha1.GcpPubSubEventSource(a.ProjectID, a.TopicID)

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

	// Make the transformer client in case the TransformerURI has been set.
	if a.TransformerURI != "" {
		a.transformer = true
		if a.transformerClient == nil {
			if a.transformerClient, err = kncloudevents.NewDefaultClient(a.TransformerURI); err != nil {
				return fmt.Errorf("failed to create transformer client: %s", err.Error())
			}
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
	// Create the CloudEvent.
	event := cloudevents.NewEvent(cloudevents.VersionV02)
	event.SetID(m.ID())
	event.SetTime(m.PublishTime())
	event.SetDataContentType(*cloudevents.StringOfApplicationJSON())
	event.SetSource(a.source)
	event.SetData(m.Message())
	event.SetType(sourcesv1alpha1.GcpPubSubSourceEventType)

	// If a transformer has been configured, then transform the message.
	if a.transformer {
		resp, err := a.transformerClient.Send(ctx, event)
		if err != nil {
			logger.Errorf("error transforming cloud event %q", event.ID())
			return err
		}
		if resp == nil {
			logger.Warnf("cloud event %q was not transformed", event.ID())
			return nil
		}
		// Update the event with the transformed one.
		event = *resp
	}

	_, err := a.ceClient.Send(ctx, event)
	return err // err could be nil or an error
}
