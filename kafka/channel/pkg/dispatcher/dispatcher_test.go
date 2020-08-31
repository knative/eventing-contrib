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

package dispatcher

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"testing"

	"knative.dev/eventing/pkg/channel/fanout"

	"github.com/Shopify/sarama"
	"github.com/cloudevents/sdk-go/v2/binding"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"go.uber.org/zap"
	"go.uber.org/zap/zaptest"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"

	eventingchannels "knative.dev/eventing/pkg/channel"
	_ "knative.dev/pkg/system/testing"

	"knative.dev/eventing-contrib/kafka/channel/pkg/utils"
	"knative.dev/eventing-contrib/kafka/common/pkg/kafka"
)

// ----- Mocks

type mockKafkaConsumerFactory struct {
	// createErr will return an error when creating a consumer
	createErr bool
}

func (c mockKafkaConsumerFactory) StartConsumerGroup(groupID string, topics []string, logger *zap.SugaredLogger, handler kafka.KafkaConsumerHandler) (sarama.ConsumerGroup, error) {
	if c.createErr {
		return nil, errors.New("error creating consumer")
	}

	return mockConsumerGroup{}, nil
}

var _ kafka.KafkaConsumerGroupFactory = (*mockKafkaConsumerFactory)(nil)

type mockConsumerGroup struct{}

func (m mockConsumerGroup) Consume(ctx context.Context, topics []string, handler sarama.ConsumerGroupHandler) error {
	return nil
}

func (m mockConsumerGroup) Errors() <-chan error {
	return nil
}

func (m mockConsumerGroup) Close() error {
	return nil
}

var _ sarama.ConsumerGroup = (*mockConsumerGroup)(nil)

// ----- Tests

// test util for various config checks
func (d *KafkaDispatcher) checkConfigAndUpdate(config *Config) error {
	if config == nil {
		return errors.New("nil config")
	}

	if _, err := d.UpdateKafkaConsumers(config); err != nil {
		// failed to update dispatchers consumers
		return err
	}
	if err := d.UpdateHostToChannelMap(config); err != nil {
		return err
	}

	return nil
}

func TestDispatcher_UpdateConfig(t *testing.T) {
	subscriber, _ := url.Parse("http://test/subscriber")

	testCases := []struct {
		name             string
		oldConfig        *Config
		newConfig        *Config
		subscribes       []string
		unsubscribes     []string
		createErr        string
		oldHostToChanMap map[string]eventingchannels.ChannelReference
		newHostToChanMap map[string]eventingchannels.ChannelReference
	}{
		{
			name:             "nil config",
			oldConfig:        &Config{},
			newConfig:        nil,
			createErr:        "nil config",
			oldHostToChanMap: map[string]eventingchannels.ChannelReference{},
			newHostToChanMap: map[string]eventingchannels.ChannelReference{},
		},
		{
			name:             "same config",
			oldConfig:        &Config{},
			newConfig:        &Config{},
			oldHostToChanMap: map[string]eventingchannels.ChannelReference{},
			newHostToChanMap: map[string]eventingchannels.ChannelReference{},
		},
		{
			name:      "config with no subscription",
			oldConfig: &Config{},
			newConfig: &Config{
				ChannelConfigs: []ChannelConfig{
					{
						Namespace: "default",
						Name:      "test-channel",
						HostName:  "a.b.c.d",
					},
				},
			},
			oldHostToChanMap: map[string]eventingchannels.ChannelReference{},
			newHostToChanMap: map[string]eventingchannels.ChannelReference{
				"a.b.c.d": {Name: "test-channel", Namespace: "default"},
			},
		},
		{
			name:      "single channel w/ new subscriptions",
			oldConfig: &Config{},
			newConfig: &Config{
				ChannelConfigs: []ChannelConfig{
					{
						Namespace: "default",
						Name:      "test-channel",
						HostName:  "a.b.c.d",
						Subscriptions: []Subscription{
							{
								UID: "subscription-1",
								Subscription: fanout.Subscription{
									Subscriber: subscriber,
								},
							},
							{
								UID: "subscription-2",
								Subscription: fanout.Subscription{
									Subscriber: subscriber,
								},
							},
						},
					},
				},
			},
			subscribes:       []string{"subscription-1", "subscription-2"},
			oldHostToChanMap: map[string]eventingchannels.ChannelReference{},
			newHostToChanMap: map[string]eventingchannels.ChannelReference{
				"a.b.c.d": {Name: "test-channel", Namespace: "default"},
			},
		},
		{
			name: "single channel w/ existing subscriptions",
			oldConfig: &Config{
				ChannelConfigs: []ChannelConfig{
					{
						Namespace: "default",
						Name:      "test-channel",
						HostName:  "a.b.c.d",
						Subscriptions: []Subscription{
							{
								UID: "subscription-1",
								Subscription: fanout.Subscription{
									Subscriber: subscriber,
								},
							},
							{
								UID: "subscription-2",
								Subscription: fanout.Subscription{
									Subscriber: subscriber,
								},
							},
						},
					},
				},
			},
			newConfig: &Config{
				ChannelConfigs: []ChannelConfig{
					{
						Namespace: "default",
						Name:      "test-channel",
						HostName:  "a.b.c.d",
						Subscriptions: []Subscription{
							{
								UID: "subscription-2",
								Subscription: fanout.Subscription{
									Subscriber: subscriber,
								},
							},
							{
								UID: "subscription-3",
								Subscription: fanout.Subscription{
									Subscriber: subscriber,
								},
							},
						},
					},
				},
			},
			subscribes:   []string{"subscription-2", "subscription-3"},
			unsubscribes: []string{"subscription-1"},
			oldHostToChanMap: map[string]eventingchannels.ChannelReference{
				"a.b.c.d": {Name: "test-channel", Namespace: "default"},
			},
			newHostToChanMap: map[string]eventingchannels.ChannelReference{
				"a.b.c.d": {Name: "test-channel", Namespace: "default"},
			},
		},
		{
			name: "multi channel w/old and new subscriptions",
			oldConfig: &Config{
				ChannelConfigs: []ChannelConfig{
					{
						Namespace: "default",
						Name:      "test-channel-1",
						HostName:  "a.b.c.d",
						Subscriptions: []Subscription{
							{
								UID: "subscription-1",
								Subscription: fanout.Subscription{
									Subscriber: subscriber,
								},
							},
							{
								UID: "subscription-2",
								Subscription: fanout.Subscription{
									Subscriber: subscriber,
								},
							},
						},
					},
				},
			},
			newConfig: &Config{
				ChannelConfigs: []ChannelConfig{
					{
						Namespace: "default",
						Name:      "test-channel-1",
						HostName:  "a.b.c.d",
						Subscriptions: []Subscription{
							{
								UID: "subscription-1",
								Subscription: fanout.Subscription{
									Subscriber: subscriber,
								},
							},
						},
					},
					{
						Namespace: "default",
						Name:      "test-channel-2",
						HostName:  "e.f.g.h",
						Subscriptions: []Subscription{
							{
								UID: "subscription-3",
								Subscription: fanout.Subscription{
									Subscriber: subscriber,
								},
							},
							{
								UID: "subscription-4",
								Subscription: fanout.Subscription{
									Subscriber: subscriber,
								},
							},
						},
					},
				},
			},
			subscribes:   []string{"subscription-1", "subscription-3", "subscription-4"},
			unsubscribes: []string{"subscription-2"},
			oldHostToChanMap: map[string]eventingchannels.ChannelReference{
				"a.b.c.d": {Name: "test-channel-1", Namespace: "default"},
			},
			newHostToChanMap: map[string]eventingchannels.ChannelReference{
				"a.b.c.d": {Name: "test-channel-1", Namespace: "default"},
				"e.f.g.h": {Name: "test-channel-2", Namespace: "default"},
			},
		},
		{
			name:      "Duplicate hostnames",
			oldConfig: &Config{},
			newConfig: &Config{
				ChannelConfigs: []ChannelConfig{
					{
						Namespace: "default",
						Name:      "test-channel-1",
						HostName:  "a.b.c.d",
					},
					{
						Namespace: "default",
						Name:      "test-channel-2",
						HostName:  "a.b.c.d",
					},
				},
			},
			createErr:        "duplicate hostName found. Each channel must have a unique host header. HostName:a.b.c.d, channel:default.test-channel-2, channel:default.test-channel-1",
			oldHostToChanMap: map[string]eventingchannels.ChannelReference{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Logf("Running %s", t.Name())
			d := &KafkaDispatcher{
				kafkaConsumerFactory: &mockKafkaConsumerFactory{},
				channelSubscriptions: make(map[eventingchannels.ChannelReference][]types.UID),
				subsConsumerGroups:   make(map[types.UID]sarama.ConsumerGroup),
				subscriptions:        make(map[types.UID]Subscription),
				topicFunc:            utils.TopicName,
				logger:               zaptest.NewLogger(t).Sugar(),
			}
			d.setHostToChannelMap(map[string]eventingchannels.ChannelReference{})

			// Initialize using oldConfig
			err := d.checkConfigAndUpdate(tc.oldConfig)
			if err != nil {

				t.Errorf("unexpected error: %v", err)
			}
			oldSubscribers := sets.NewString()
			for _, sub := range d.subscriptions {
				oldSubscribers.Insert(string(sub.UID))
			}
			if diff := sets.NewString(tc.unsubscribes...).Difference(oldSubscribers); diff.Len() != 0 {
				t.Errorf("subscriptions %+v were never subscribed", diff)
			}
			if diff := cmp.Diff(tc.oldHostToChanMap, d.getHostToChannelMap()); diff != "" {
				t.Errorf("unexpected hostToChannelMap (-want, +got) = %v", diff)
			}

			// Update with new config
			err = d.checkConfigAndUpdate(tc.newConfig)
			if tc.createErr != "" {
				if err == nil {
					t.Errorf("Expected UpdateConfig error: '%v'. Actual nil", tc.createErr)
				} else if err.Error() != tc.createErr {
					t.Errorf("Unexpected UpdateConfig error. Expected '%v'. Actual '%v'", tc.createErr, err)
				}
				return
			} else if err != nil {
				t.Errorf("Unexpected UpdateConfig error. Expected nil. Actual '%v'", err)
			}

			var newSubscribers []string
			for id := range d.subscriptions {
				newSubscribers = append(newSubscribers, string(id))
			}

			if diff := cmp.Diff(tc.subscribes, newSubscribers, sortStrings); diff != "" {
				t.Errorf("unexpected subscribers (-want, +got) = %v", diff)
			}
			if diff := cmp.Diff(tc.newHostToChanMap, d.getHostToChannelMap()); diff != "" {
				t.Errorf("unexpected hostToChannelMap (-want, +got) = %v", diff)
			}

		})
	}
}

func TestSubscribeError(t *testing.T) {
	cf := &mockKafkaConsumerFactory{createErr: true}
	d := &KafkaDispatcher{
		kafkaConsumerFactory: cf,
		logger:               zap.NewNop().Sugar(),
		topicFunc:            utils.TopicName,
	}

	channelRef := eventingchannels.ChannelReference{
		Name:      "test-channel",
		Namespace: "test-ns",
	}

	subRef := Subscription{
		UID:          "test-sub",
		Subscription: fanout.Subscription{},
	}
	err := d.subscribe(channelRef, subRef)
	if err == nil {
		t.Errorf("Expected error want %s, got %s", "error creating consumer", err)
	}
}

func TestUnsubscribeUnknownSub(t *testing.T) {
	cf := &mockKafkaConsumerFactory{createErr: true}
	d := &KafkaDispatcher{
		kafkaConsumerFactory: cf,
		logger:               zap.NewNop().Sugar(),
	}

	channelRef := eventingchannels.ChannelReference{
		Name:      "test-channel",
		Namespace: "test-ns",
	}

	subRef := Subscription{
		UID:          "test-sub",
		Subscription: fanout.Subscription{},
	}
	if err := d.unsubscribe(channelRef, subRef); err != nil {
		t.Errorf("Unsubscribe error: %v", err)
	}
}

func TestKafkaDispatcher_Start(t *testing.T) {
	d := &KafkaDispatcher{}

	err := d.Start(context.TODO())
	if err == nil {
		t.Errorf("Expected error want %s, got %s", "message receiver is not set", err)
	}

	receiver, err := eventingchannels.NewMessageReceiver(
		func(ctx context.Context, channel eventingchannels.ChannelReference, message binding.Message, _ []binding.Transformer, _ http.Header) error {
			return nil
		},
		zap.NewNop(),
		eventingchannels.ResolveMessageChannelFromHostHeader(d.getChannelReferenceFromHost))
	if err != nil {
		t.Fatalf("Error creating new message receiver. Error:%s", err)
	}
	d.receiver = receiver
	err = d.Start(context.TODO())
	if err == nil {
		t.Errorf("Expected error want %s, got %s", "kafkaAsyncProducer is not set", err)
	}
}

func TestNewDispatcher(t *testing.T) {
	args := &KafkaDispatcherArgs{
		ClientID:  "kafka-ch-dispatcher",
		Brokers:   []string{"localhost:10000"},
		TopicFunc: utils.TopicName,
		Logger:    nil,
	}
	_, err := NewDispatcher(context.TODO(), args)
	if err == nil {
		t.Errorf("Expected error want %s, got %s", "message receiver is not set", err)
	}
}

var sortStrings = cmpopts.SortSlices(func(x, y string) bool {
	return x < y
})
