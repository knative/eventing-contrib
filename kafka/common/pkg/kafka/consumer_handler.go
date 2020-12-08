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
	// When this function returns true, the consumer group offset is marked as consumed.
	// The returned error is enqueued in errors channel.
	Handle(context context.Context, message *sarama.ConsumerMessage) (bool, error)
}

// ConsumerHandler implements sarama.ConsumerGroupHandler and provides some glue code to simplify message handling
// You must implement KafkaConsumerHandler and create a new SaramaConsumerHandler with it
type SaramaConsumerHandler struct {
	// The user message handler
	handler KafkaConsumerHandler

	logger *zap.SugaredLogger

	// Errors channel
	errors chan error
}

func NewConsumerHandler(logger *zap.SugaredLogger, handler KafkaConsumerHandler, errorsCh chan error) SaramaConsumerHandler {
	return SaramaConsumerHandler{
		logger:  logger,
		handler: handler,
		errors:  errorsCh,
	}
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (consumer *SaramaConsumerHandler) Setup(sarama.ConsumerGroupSession) error {
	consumer.logger.Info("setting up handler")
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (consumer *SaramaConsumerHandler) Cleanup(session sarama.ConsumerGroupSession) error {
	consumer.logger.Info("cleanup handler")
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (consumer *SaramaConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	consumer.logger.Info(fmt.Sprintf("Starting partition consumer, topic: %s, partition: %d, initialOffset: %d", claim.Topic(), claim.Partition(), claim.InitialOffset()))

	// NOTE:
	// Do not move the code below to a goroutine.
	// The `ConsumeClaim` itself is called within a goroutine, see:
	// https://github.com/Shopify/sarama/blob/master/consumer_group.go#L27-L29
	for message := range claim.Messages() {
		if ce := consumer.logger.Desugar().Check(zap.DebugLevel, "debugging"); ce != nil {
			consumer.logger.Debugw("Message claimed", zap.String("topic", message.Topic), zap.Binary("value", message.Value))
		}

		// Don't use the session context since it is closed before messages are drained.
		// Handle must finish before the session timeout.
		mustMark, err := consumer.handler.Handle(context.Background(), message)

		if err != nil {
			consumer.logger.Infow("Failure while handling a message", zap.String("topic", message.Topic), zap.Int32("partition", message.Partition), zap.Int64("offset", message.Offset), zap.Error(err))
			consumer.errors <- err
		}

		if mustMark {
			session.MarkMessage(message, "") // Mark kafka message as processed
			if ce := consumer.logger.Desugar().Check(zap.DebugLevel, "debugging"); ce != nil {
				consumer.logger.Debugw("Message marked", zap.String("topic", message.Topic), zap.Binary("value", message.Value))
			}
		}

	}

	consumer.logger.Infof("Stopping partition consumer, topic: %s, partition: %d", claim.Topic(), claim.Partition())
	return nil
}

var _ sarama.ConsumerGroupHandler = (*SaramaConsumerHandler)(nil)
