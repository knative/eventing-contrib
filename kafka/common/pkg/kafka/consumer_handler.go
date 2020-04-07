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
	"errors"
	"fmt"
	"time"

	"github.com/Shopify/sarama"
	"k8s.io/apimachinery/pkg/util/wait"

	"go.uber.org/zap"
)

var (
	PermanentError = errors.New("unrecoverable error")

	// TODO make configurable
	defaultBackoff = wait.Backoff{
		Steps:    20,
		Duration: 10 * time.Millisecond,
		Factor:   2.0,
		Jitter:   0.1,
	}
)

type KafkaConsumerHandler interface {
	// When this function returns true, the consumer group offset is committed.
	// The returned error is enqueued in errors channel.
	Handle(context context.Context, message *sarama.ConsumerMessage) (bool, error)
}

// saramaConsumerHandler implements sarama.ConsumerGroupHandler and provides some glue code to simplify message handling
type saramaConsumerHandler struct {
	// The user message handler
	handler KafkaConsumerHandler

	backoff wait.Backoff

	logger *zap.Logger
	// Errors channel
	errors chan error
}

func NewConsumerHandler(logger *zap.Logger, handler KafkaConsumerHandler) saramaConsumerHandler {
	return NewConsumerHandlerWithBackoff(logger, handler, defaultBackoff)
}

func NewConsumerHandlerWithBackoff(logger *zap.Logger, handler KafkaConsumerHandler, backoff wait.Backoff) saramaConsumerHandler {
	return saramaConsumerHandler{
		handler: handler,
		backoff: backoff,
		logger:  logger,
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
	consumer.logger.Info(fmt.Sprintf("Starting partition consumer, topic: %s, partition: %d, initialOffset: %d", claim.Topic(), claim.Partition(), claim.InitialOffset()))
	defer consumer.logger.Info(fmt.Sprintf("Stopping partition consumer, topic: %s, partition: %d", claim.Topic(), claim.Partition()))

	// NOTE: Do not move the code below to a goroutine. The `ConsumeClaim` itself is called within a goroutine.
	for message := range claim.Messages() {
		if ce := consumer.logger.Check(zap.DebugLevel, "debugging"); ce != nil {
			consumer.logger.Debug("Message claimed", zap.String("topic", message.Topic), zap.Binary("value", message.Value))
			consumer.logger.Debug("message", zap.String("topic", message.Topic), zap.Int32("partition", message.Partition), zap.Int64("Offset", message.Offset))
		}

		err := consumer.handleWithExponentialBackOffRetries(session, message)
		if err != nil {
			consumer.errors <- PermanentError
		}
	}

	return nil
}

func (consumer *saramaConsumerHandler) handleWithExponentialBackOffRetries(
	session sarama.ConsumerGroupSession,
	message *sarama.ConsumerMessage,
) error {
	return wait.ExponentialBackoff(consumer.backoff, func() (bool, error) {

		shouldMark, err := consumer.handler.Handle(session.Context(), message)

		if shouldMark {
			session.MarkMessage(message, "") // Mark kafka message as processed
			if ce := consumer.logger.Check(zap.DebugLevel, "debugging"); ce != nil {
				consumer.logger.Debug("Message marked", zap.String("topic", message.Topic), zap.Binary("value", message.Value))
			}
		}

		if err != nil {
			consumer.logger.Info("Failure while handling a message", zap.String("topic", message.Topic), zap.Int32("partition", message.Partition), zap.Int64("offset", message.Offset), zap.Error(err))
			consumer.errors <- err
			return false, nil
		}

		return true, nil
	})
}

var _ sarama.ConsumerGroupHandler = (*saramaConsumerHandler)(nil)
