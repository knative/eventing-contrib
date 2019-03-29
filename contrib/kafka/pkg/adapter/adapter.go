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

package kafka

import (
	"encoding/json"
	"strconv"
	"strings"

	"github.com/Shopify/sarama"
	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"
	"github.com/knative/eventing-sources/pkg/kncloudevents"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	"golang.org/x/net/context"
)

const (
	eventType = "dev.knative.kafka.event"
)

type AdapterSASL struct {
	Enable   bool
	User     string
	Password string
}

type AdapterTLS struct {
	Enable bool
}

type AdapterNet struct {
	SASL AdapterSASL
	TLS  AdapterTLS
}

type Adapter struct {
	BootstrapServers string
	Topics           string
	ConsumerGroup    string
	Net              AdapterNet
	SinkURI          string
	client           client.Client
}

// --------------------------------------------------------------------

// ConsumerGroupHandler functions to define message consume and related logic.
func (a *Adapter) Setup(_ sarama.ConsumerGroupSession) error {
	if a.client == nil {
		var err error
		if a.client, err = kncloudevents.NewDefaultClient(a.SinkURI); err != nil {
			return err
		}
	}
	return nil
}
func (a *Adapter) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }
func (a *Adapter) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {

	logger := logging.FromContext(context.TODO())

	for msg := range claim.Messages() {
		logger.Info("Received: ", zap.Any("value", string(msg.Value)))

		go func(msg *sarama.ConsumerMessage) {
			// send and mark message if post was successful
			if err := a.postMessage(context.TODO(), msg); err == nil {
				sess.MarkMessage(msg, "")
				logger.Debug("Successfully sent event to sink")
			} else {
				logger.Error("Sending event to sink failed: ", zap.Error(err))
			}
		}(msg)
	}
	return nil
}

// --------------------------------------------------------------------

func (a *Adapter) Start(ctx context.Context, stopCh <-chan struct{}) error {
	logger := logging.FromContext(ctx)

	logger.Info("Starting with config: ", zap.Any("adapter", a))

	kafkaConfig := sarama.NewConfig()
	kafkaConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
	kafkaConfig.Version = sarama.V2_0_0_0
	kafkaConfig.Consumer.Return.Errors = true
	kafkaConfig.Net.SASL.Enable = a.Net.SASL.Enable
	kafkaConfig.Net.SASL.User = a.Net.SASL.User
	kafkaConfig.Net.SASL.Password = a.Net.SASL.Password
	kafkaConfig.Net.TLS.Enable = a.Net.TLS.Enable

	// Start with a client
	client, err := sarama.NewClient(strings.Split(a.BootstrapServers, ","), kafkaConfig)
	if err != nil {
		panic(err)
	}
	defer func() { _ = client.Close() }()

	// init consumer group
	group, err := sarama.NewConsumerGroupFromClient(a.ConsumerGroup, client)
	if err != nil {
		panic(err)
	}
	defer func() { _ = group.Close() }()

	// Track errors
	go func() {
		for err := range group.Errors() {
			logger.Error("ERROR", err)
		}
	}()

	// Handle session
	go func() {
		for {
			if err := group.Consume(ctx, strings.Split(a.Topics, ","), a); err != nil {
				panic(err)
			}
		}
	}()

	for {
		select {
		case <-stopCh:
			logger.Info("Shutting down...")
			return nil
		}
	}
}

func (a *Adapter) postMessage(ctx context.Context, msg *sarama.ConsumerMessage) error {

	extensions := map[string]interface{}{
		"key": string(msg.Key),
	}
	event := cloudevents.Event{
		Context: cloudevents.EventContextV02{
			SpecVersion: cloudevents.CloudEventsVersionV02,
			Type:        eventType,
			ID:          "partition:" + strconv.Itoa(int(msg.Partition)) + "/offset:" + strconv.FormatInt(msg.Offset, 10),
			Time:        &types.Timestamp{Time: msg.Timestamp},
			Source:      *types.ParseURLRef(msg.Topic),
			ContentType: cloudevents.StringOfApplicationJSON(),
			Extensions:  extensions,
		}.AsV02(),
		Data: a.jsonEncode(ctx, msg.Value),
	}

	_, err := a.client.Send(ctx, event)
	return err
}

func (a *Adapter) jsonEncode(ctx context.Context, value []byte) interface{} {
	var payload map[string]interface{}

	logger := logging.FromContext(ctx)

	if err := json.Unmarshal(value, &payload); err != nil {
		logger.Info("Error unmarshalling JSON: ", zap.Error(err))
		return value
	} else {
		return payload
	}
}
