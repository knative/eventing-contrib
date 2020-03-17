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
	"io/ioutil"
	"net/http"
	"testing"

	"knative.dev/pkg/apis"

	"github.com/Shopify/sarama"
	cloudevents "github.com/cloudevents/sdk-go/legacy"
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	"go.uber.org/zap"
	"k8s.io/apimachinery/pkg/util/sets"

	"knative.dev/eventing-contrib/kafka/channel/pkg/utils"
	"knative.dev/eventing-contrib/kafka/common/pkg/kafka"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	eventingchannels "knative.dev/eventing/pkg/channel"
	"knative.dev/eventing/pkg/channel/fanout"
	"knative.dev/eventing/pkg/channel/multichannelfanout"
	_ "knative.dev/pkg/system/testing"
)

//----- Mocks

type mockKafkaConsumerFactory struct {
	// createErr will return an error when creating a consumer
	createErr bool
}

func (c mockKafkaConsumerFactory) StartConsumerGroup(groupID string, topics []string, logger *zap.Logger, handler kafka.KafkaConsumerHandler) (sarama.ConsumerGroup, error) {
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

//----- Tests

// test util for various config checks
func (d *KafkaDispatcher) checkConfigAndUpdate(config *multichannelfanout.Config) error {
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
	subscriber, _ := apis.ParseURL("http://test/subscriber")

	testCases := []struct {
		name             string
		oldConfig        *multichannelfanout.Config
		newConfig        *multichannelfanout.Config
		subscribes       []string
		unsubscribes     []string
		createErr        string
		oldHostToChanMap map[string]eventingchannels.ChannelReference
		newHostToChanMap map[string]eventingchannels.ChannelReference
	}{
		{
			name:             "nil config",
			oldConfig:        &multichannelfanout.Config{},
			newConfig:        nil,
			createErr:        "nil config",
			oldHostToChanMap: map[string]eventingchannels.ChannelReference{},
			newHostToChanMap: map[string]eventingchannels.ChannelReference{},
		},
		{
			name:             "same config",
			oldConfig:        &multichannelfanout.Config{},
			newConfig:        &multichannelfanout.Config{},
			oldHostToChanMap: map[string]eventingchannels.ChannelReference{},
			newHostToChanMap: map[string]eventingchannels.ChannelReference{},
		},
		{
			name:      "config with no subscription",
			oldConfig: &multichannelfanout.Config{},
			newConfig: &multichannelfanout.Config{
				ChannelConfigs: []multichannelfanout.ChannelConfig{
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
			oldConfig: &multichannelfanout.Config{},
			newConfig: &multichannelfanout.Config{
				ChannelConfigs: []multichannelfanout.ChannelConfig{
					{
						Namespace: "default",
						Name:      "test-channel",
						HostName:  "a.b.c.d",
						FanoutConfig: fanout.Config{
							Subscriptions: []eventingduck.SubscriberSpec{
								{
									UID:           "subscription-1",
									SubscriberURI: subscriber,
								},
								{
									UID:           "subscription-2",
									SubscriberURI: subscriber,
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
			oldConfig: &multichannelfanout.Config{
				ChannelConfigs: []multichannelfanout.ChannelConfig{
					{
						Namespace: "default",
						Name:      "test-channel",
						HostName:  "a.b.c.d",
						FanoutConfig: fanout.Config{
							Subscriptions: []eventingduck.SubscriberSpec{
								{
									UID:           "subscription-1",
									SubscriberURI: subscriber,
								},
								{
									UID:           "subscription-2",
									SubscriberURI: subscriber,
								}}}}},
			},
			newConfig: &multichannelfanout.Config{
				ChannelConfigs: []multichannelfanout.ChannelConfig{
					{
						Namespace: "default",
						Name:      "test-channel",
						HostName:  "a.b.c.d",
						FanoutConfig: fanout.Config{
							Subscriptions: []eventingduck.SubscriberSpec{
								{
									UID:           "subscription-2",
									SubscriberURI: subscriber,
								},
								{
									UID:           "subscription-3",
									SubscriberURI: subscriber,
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
			oldConfig: &multichannelfanout.Config{
				ChannelConfigs: []multichannelfanout.ChannelConfig{
					{
						Namespace: "default",
						Name:      "test-channel-1",
						HostName:  "a.b.c.d",
						FanoutConfig: fanout.Config{
							Subscriptions: []eventingduck.SubscriberSpec{
								{
									UID:           "subscription-1",
									SubscriberURI: subscriber,
								},
								{
									UID:           "subscription-2",
									SubscriberURI: subscriber,
								},
							},
						}}}},
			newConfig: &multichannelfanout.Config{
				ChannelConfigs: []multichannelfanout.ChannelConfig{
					{
						Namespace: "default",
						Name:      "test-channel-1",
						HostName:  "a.b.c.d",
						FanoutConfig: fanout.Config{
							Subscriptions: []eventingduck.SubscriberSpec{
								{
									UID:           "subscription-1",
									SubscriberURI: subscriber,
								},
							},
						},
					},
					{
						Namespace: "default",
						Name:      "test-channel-2",
						HostName:  "e.f.g.h",
						FanoutConfig: fanout.Config{
							Subscriptions: []eventingduck.SubscriberSpec{
								{
									UID:           "subscription-3",
									SubscriberURI: subscriber,
								},
								{
									UID:           "subscription-4",
									SubscriberURI: subscriber,
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
			oldConfig: &multichannelfanout.Config{},
			newConfig: &multichannelfanout.Config{
				ChannelConfigs: []multichannelfanout.ChannelConfig{
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
				kafkaConsumerGroups:  make(map[eventingchannels.ChannelReference]map[subscription]sarama.ConsumerGroup),
				topicFunc:            utils.TopicName,
				logger:               zap.NewNop(),
			}
			d.setConfig(&multichannelfanout.Config{})
			d.setHostToChannelMap(map[string]eventingchannels.ChannelReference{})

			// Initialize using oldConfig
			err := d.checkConfigAndUpdate(tc.oldConfig)
			if err != nil {

				t.Errorf("unexpected error: %v", err)
			}
			oldSubscribers := sets.NewString()
			for _, subMap := range d.kafkaConsumerGroups {
				for sub := range subMap {
					oldSubscribers.Insert(sub.UID)
				}
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
			for _, subMap := range d.kafkaConsumerGroups {
				for sub := range subMap {
					newSubscribers = append(newSubscribers, sub.UID)
				}
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

func TestFromKafkaMessage(t *testing.T) {
	data := []byte("data")
	ctx := context.Background()
	kafkaMessage := &sarama.ConsumerMessage{
		Headers: []*sarama.RecordHeader{
			{
				Key:   []byte("ce_k1"),
				Value: []byte("v1"),
			},
			{
				Key:   []byte("ce_id"),
				Value: []byte("im-a-snowflake"),
			},
			{
				Key:   []byte("ce_source"),
				Value: []byte("testsource"),
			},
			{
				Key:   []byte("ce_type"),
				Value: []byte("testtype"),
			},
			{
				Key:   []byte("ce_extensionfield"),
				Value: []byte("testextension"),
			},
			{
				Key:   []byte("anotherkafkaheader"),
				Value: []byte("notpassedthrough"),
			},
		},
		Value: data,
	}
	want := cloudevents.NewEvent(cloudevents.VersionV1)
	want.SetExtension("k1", "v1")
	want.SetSource("testsource")
	want.SetType("testtype")
	want.SetID("im-a-snowflake")
	want.SetExtension("extensionfield", "testextension")
	want.SetData(data)

	var got *cloudevents.Event
	got = fromKafkaMessage(ctx, kafkaMessage)
	if diff := cmp.Diff(&want, got); diff != "" {
		t.Errorf("unexpected message (-want, +got) = %s", diff)
	}
}

func TestToKafkaMessage(t *testing.T) {
	data := []byte("data")

	channelRef := eventingchannels.ChannelReference{
		Name:      "test-channel",
		Namespace: "test-ns",
	}

	event := cloudevents.NewEvent(cloudevents.VersionV1)
	event.SetExtension("k1", "v1")
	event.SetID("im-a-snowflake")
	event.SetSource("testsource")
	event.SetType("testtype")
	event.SetData(data)

	want := &sarama.ProducerMessage{
		Topic: "knative-messaging-kafka.test-ns.test-channel",
		Headers: []sarama.RecordHeader{
			{
				Key:   []byte("ce_specversion"),
				Value: []byte("1.0"),
			},
			{
				Key:   []byte("ce_type"),
				Value: []byte("testtype"),
			},
			{
				Key:   []byte("ce_source"),
				Value: []byte("testsource"),
			},
			{
				Key:   []byte("ce_id"),
				Value: []byte("im-a-snowflake"),
			},
			{
				Key:   []byte("ce_time"),
				Value: []byte("ignoreme"),
			},
			{
				Key:   []byte("ce_k1"),
				Value: []byte("v1"),
			},
		},
		Value: sarama.ByteEncoder(data),
	}

	got := toKafkaMessage(context.TODO(), channelRef, event, utils.TopicName)
	got.Headers[4] = sarama.RecordHeader{
		Key:   []byte("ce_time"),
		Value: []byte("ignoreme"),
	}
	if diff := cmp.Diff(want, got, cmpopts.IgnoreUnexported(sarama.ProducerMessage{})); diff != "" {
		t.Errorf("unexpected message (-want, +got) = %s", diff)
	}
}

type dispatchTestHandler struct {
	t       *testing.T
	payload []byte
	done    chan bool
}

func (h *dispatchTestHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	defer r.Body.Close()
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		h.t.Error("Failed to read the request body")
	}
	if diff := cmp.Diff(h.payload, body); diff != "" {
		h.t.Errorf("unexpected body (-want, +got) = %s", diff)
	}
	h.done <- true
}

func TestSubscribeError(t *testing.T) {
	cf := &mockKafkaConsumerFactory{createErr: true}
	d := &KafkaDispatcher{
		kafkaConsumerFactory: cf,
		logger:               zap.NewNop(),
		topicFunc:            utils.TopicName,
	}

	channelRef := eventingchannels.ChannelReference{
		Name:      "test-channel",
		Namespace: "test-ns",
	}

	subRef := subscription{
		UID: "test-sub",
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
		logger:               zap.NewNop(),
	}

	channelRef := eventingchannels.ChannelReference{
		Name:      "test-channel",
		Namespace: "test-ns",
	}

	subRef := subscription{
		UID: "test-sub",
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

	receiver, err := eventingchannels.NewEventReceiver(func(ctx context.Context, channel eventingchannels.ChannelReference, event cloudevents.Event) error {
		return nil
	}, zap.NewNop(),
		eventingchannels.ResolveChannelFromHostHeader(eventingchannels.ResolveChannelFromHostFunc(d.getChannelReferenceFromHost)))
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
		Brokers:   []string{"127.0.0.1:9092"},
		TopicFunc: utils.TopicName,
		Logger:    nil,
	}
	_, err := NewDispatcher(args)
	if err == nil {
		t.Errorf("Expected error want %s, got %s", "message receiver is not set", err)
	}
}

var sortStrings = cmpopts.SortSlices(func(x, y string) bool {
	return x < y
})
