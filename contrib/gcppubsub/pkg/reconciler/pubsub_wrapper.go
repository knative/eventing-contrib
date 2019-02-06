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
	"context"
	"fmt"

	"cloud.google.com/go/pubsub"
)

// This file exists so that we can unit test failures with the PubSub client.

// pubSubClientCreator creates a pubSubClient.
type pubSubClientCreator func(ctx context.Context, googleCloudProject string) (pubSubClient, error)

// pubSubClient is the set of methods we use on pubsub.Client.
type pubSubClient interface {
	SubscriptionInProject(id, projectId string) pubSubSubscription
	CreateSubscription(ctx context.Context, id string, cfg pubsub.SubscriptionConfig) (pubSubSubscription, error)
	Topic(name string) *pubsub.Topic
}

// realGcpPubSubClient is the client that will be used everywhere except unit tests. Verify that it
// satisfies the interface.
var _ pubSubClient = &realGcpPubSubClient{}

// realGcpPubSubClient wraps a real GCP PubSub client, so that it matches the pubSubClient
// interface. It is needed because the real SubscriptionInProject returns a struct and does not
// implicitly match gcpPubSubClient, which returns an interface.
type realGcpPubSubClient struct {
	client *pubsub.Client
}

func (c *realGcpPubSubClient) SubscriptionInProject(id, projectId string) pubSubSubscription {
	if c.client != nil {
		return c.client.SubscriptionInProject(id, projectId)
	}
	return nil
}

func (c *realGcpPubSubClient) CreateSubscription(ctx context.Context, id string, cfg pubsub.SubscriptionConfig) (pubSubSubscription, error) {
	if c.client != nil {
		return c.client.CreateSubscription(ctx, id, cfg)
	}
	return nil, fmt.Errorf("pubsub client is nil")
}

func (c *realGcpPubSubClient) Topic(id string) *pubsub.Topic {
	if c.client != nil {
		return c.client.Topic(id)
	}
	return nil
}

// pubSubSubscription is the set of methods we use on pubsub.Subscription. It exists to make
// pubSubClient unit testable.
type pubSubSubscription interface {
	Exists(ctx context.Context) (bool, error)
	ID() string
	Delete(ctx context.Context) error
}

// pubsub.Subscription is the real pubSubSubscription that is used everywhere except unit tests.
// Verify that it satisfies the interface.
var _ pubSubSubscription = &pubsub.Subscription{}
