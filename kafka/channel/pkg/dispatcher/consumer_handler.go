package dispatcher

import (
	"fmt"
	"github.com/Shopify/sarama"
	"go.uber.org/zap"
	"knative.dev/eventing/pkg/provisioners"
)

type KafkaConsumerHandler struct {
	channelRef provisioners.ChannelReference
	sub subscription
	logger *zap.Logger
	dispatcher *provisioners.MessageDispatcher
}

// Setup is run at the beginning of a new session, before ConsumeClaim
func (consumer *KafkaConsumerHandler) Setup(sarama.ConsumerGroupSession) error {
	// Mark the consumer as ready
	consumer.logger.Info("Consumer for subscription started", zap.Any("channelRef", consumer.channelRef), zap.Any("subscription", consumer.sub))
	return nil
}

// Cleanup is run at the end of a session, once all ConsumeClaim goroutines have exited
func (consumer *KafkaConsumerHandler) Cleanup(sarama.ConsumerGroupSession) error {
	consumer.logger.Info("Consumer for subscription stopped", zap.Any("channelRef", consumer.channelRef), zap.Any("subscription", consumer.sub))
	return nil
}

// ConsumeClaim must start a consumer loop of ConsumerGroupClaim's Messages().
func (consumer *KafkaConsumerHandler) ConsumeClaim(session sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {
	consumer.logger.Info("Starting consuming consumer group claim", zap.Any("channelRef", consumer.channelRef), zap.Any("subscription", consumer.sub))

	// NOTE:
	// Do not move the code below to a goroutine.
	// The `ConsumeClaim` itself is called within a goroutine, see:
	// https://github.com/Shopify/sarama/blob/master/consumer_group.go#L27-L29
	for message := range claim.Messages() {
		consumer.logger.Info(fmt.Sprintf("Message claimed: value = %s, timestamp = %v, topic = %s", string(message.Value), message.Timestamp, message.Topic))
		consumer.logger.Info(
			"Dispatching a knativeMessage for subscription",
			zap.Any("channelRef", consumer.channelRef),
			zap.Any("subscription", consumer.sub),
			zap.Any("partition", message.Partition),
			zap.Any("offset", message.Offset),
		)

		knativeMessage := fromKafkaMessage(message)
		err := consumer.dispatchMessage(knativeMessage)
		if err != nil {
			consumer.logger.Warn("Got error trying to dispatch knativeMessage", zap.Error(err))
		}

		session.MarkMessage(message, "") // Mark knativeMessage as processed
	}

	return nil
}

// dispatchMessage sends the request to exactly one subscription. It handles both the `call` and
// the `sink` portions of the subscription.
func (consumer *KafkaConsumerHandler) dispatchMessage(m *provisioners.Message) error {
	return consumer.dispatcher.DispatchMessage(m, consumer.sub.SubscriberURI, consumer.sub.ReplyURI, provisioners.DispatchDefaults{})
}

var _ sarama.ConsumerGroupHandler = (*KafkaConsumerHandler)(nil)