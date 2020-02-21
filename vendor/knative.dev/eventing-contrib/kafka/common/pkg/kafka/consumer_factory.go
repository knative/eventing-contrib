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
	"sync"

	"github.com/Shopify/sarama"
	"go.uber.org/zap"
)

var newConsumerGroupFromClient = sarama.NewConsumerGroupFromClient

// Kafka consumer factory creates the ConsumerGroup and start consuming the specified topic
type KafkaConsumerGroupFactory interface {
	StartConsumerGroup(groupID string, topics []string, logger *zap.Logger, handler KafkaConsumerHandler) (sarama.ConsumerGroup, error)
}

type kafkaConsumerGroupFactoryImpl struct {
	client sarama.Client
}

type customConsumerGroup struct {
	handlerErrorChannel chan error
	sarama.ConsumerGroup
}

// Merge handler errors chan and consumer group error chan
func (c customConsumerGroup) Errors() <-chan error {
	errors := make(chan error, 10)
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		for e := range c.ConsumerGroup.Errors() {
			errors <- e
		}
		wg.Done()
	}()
	go func() {
		for e := range c.handlerErrorChannel {
			errors <- e
		}
		wg.Done()
	}()

	// Synchronization routine to close the error channel
	go func() {
		wg.Wait()
		close(errors)
	}()
	return errors
}

var _ sarama.ConsumerGroup = (*customConsumerGroup)(nil)

func (c kafkaConsumerGroupFactoryImpl) StartConsumerGroup(groupID string, topics []string, logger *zap.Logger, handler KafkaConsumerHandler) (sarama.ConsumerGroup, error) {
	consumerGroup, err := newConsumerGroupFromClient(groupID, c.client)

	if err != nil {
		return nil, err
	}

	consumerHandler := NewConsumerHandler(logger, handler)

	go func() {
		err2 := consumerGroup.Consume(context.TODO(), topics, &consumerHandler)
		if err2 != nil {
			consumerHandler.errors <- err2
		}
	}()

	return customConsumerGroup{consumerHandler.errors, consumerGroup}, err
}

func NewConsumerGroupFactory(client sarama.Client) KafkaConsumerGroupFactory {
	return kafkaConsumerGroupFactoryImpl{client}
}

var _ KafkaConsumerGroupFactory = (*kafkaConsumerGroupFactoryImpl)(nil)
