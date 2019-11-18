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
	"context"
	"fmt"

	"github.com/Shopify/sarama"
	"go.uber.org/zap"
)

type KafkaConsumerHandler interface {
	// When this function returns true, the consumer group offset is committed.
	// The returned error is enqueued in errors channel.
	Handle(context context.Context, message *sarama.ConsumerMessage) (bool, error)
}

// ConsumerHandler implements sarama.ConsumerGroupHandler and provides some glue code to simplify message handling
// You must implement KafkaConsumerHandler and create a new saramaConsumerHandler with it
type saramaConsumerHandler struct {
	// The user message handler
	handler KafkaConsumerHandler

	logger *zap.Logger
	// Errors channel
	errors chan error
}

func NewConsumerHandler(logger *zap.Logger, handler KafkaConsumerHandler) saramaConsumerHandler {
	return saramaConsumerHandler{
		logger:  logger,
		handler: handler,
		errors:  make(chan error, 10), // Some buffering to avoid blocking the message processing
	}
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (consumer *saramaConsumerHandler) Setup(sarama.ConsumerGroupSession) error {
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (consumer *saramaConsumerHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	close(consumer.errors)
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (consumer *saramaConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	consumer.logger.Info(fmt.Sprintf("Starting partition consumer, topic: %s, partition: %d", claim.Topic(), claim.Partition()))

	// NOTE:
	// Do not move the code below to a goroutine.
	// The `ConsumeClaim` itself is called within a goroutine, see:
	// https://github.com/Shopify/sarama/blob/master/consumer_group.go#L27-L29
	for message := range claim.Messages() {
		if ce := consumer.logger.Check(zap.DebugLevel, "debugging"); ce != nil {
			consumer.logger.Debug("Message claimed", zap.String("topic", message.Topic), zap.Binary("value", message.Value))
		}

		mustMark, err := consumer.handler.Handle(session.Context(), message)

		if err != nil {
			consumer.errors <- err
		}
		if mustMark {
			session.MarkMessage(message, "") // Mark kafka message as processed
			if ce := consumer.logger.Check(zap.DebugLevel, "debugging"); ce != nil {
				consumer.logger.Debug("Message marked", zap.String("topic", message.Topic), zap.Binary("value", message.Value))
			}
		}

	}

	consumer.logger.Info(fmt.Sprintf("Stopping partition consumer, topic: %s, partition: %d", claim.Topic(), claim.Partition()))
	return nil
}

var _ sarama.ConsumerGroupHandler = (*saramaConsumerHandler)(nil)
