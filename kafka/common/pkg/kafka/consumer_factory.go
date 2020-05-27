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

var newConsumerGroup = sarama.NewConsumerGroup

// Kafka consumer factory creates the ConsumerGroup and start consuming the specified topic
type KafkaConsumerGroupFactory interface {
	StartConsumerGroup(groupID string, topics []string, logger *zap.Logger, handler KafkaConsumerHandler) (sarama.ConsumerGroup, error)
}

type kafkaConsumerGroupFactoryImpl struct {
	config *sarama.Config
	addrs  []string
}

type customConsumerGroup struct {
	cancel              func()
	handlerErrorChannel chan error
	sarama.ConsumerGroup
}

// Merge handler errors chan and consumer group error chan
func (c *customConsumerGroup) Errors() <-chan error {
	return mergeErrorChannels(c.ConsumerGroup.Errors(), c.handlerErrorChannel)
}

func (c *customConsumerGroup) Close() error {
	c.cancel()
	return c.ConsumerGroup.Close()
}

var _ sarama.ConsumerGroup = (*customConsumerGroup)(nil)

func (c kafkaConsumerGroupFactoryImpl) StartConsumerGroup(groupID string, topics []string, logger *zap.Logger, handler KafkaConsumerHandler) (sarama.ConsumerGroup, error) {
	consumerGroup, err := newConsumerGroup(c.addrs, groupID, c.config)
	if err != nil {
		return nil, err
	}

	consumerHandler := NewConsumerHandler(logger, handler)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		for {
			err := consumerGroup.Consume(context.Background(), topics, &consumerHandler)
			if err == sarama.ErrClosedConsumerGroup {
				return
			}
			if err != nil {
				consumerHandler.errors <- err
			}

			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()

	return &customConsumerGroup{cancel, consumerHandler.errors, consumerGroup}, err
}

func NewConsumerGroupFactory(addrs []string, config *sarama.Config) KafkaConsumerGroupFactory {
	return kafkaConsumerGroupFactoryImpl{addrs: addrs, config: config}
}

var _ KafkaConsumerGroupFactory = (*kafkaConsumerGroupFactoryImpl)(nil)

func mergeErrorChannels(channels ...<-chan error) <-chan error {
	out := make(chan error)
	var wg sync.WaitGroup
	wg.Add(len(channels))
	for _, channel := range channels {
		go func(c <-chan error) {
			for v := range c {
				out <- v
			}
			wg.Done()
		}(channel)
	}
	go func() {
		wg.Wait()
		close(out)
	}()
	return out
}
