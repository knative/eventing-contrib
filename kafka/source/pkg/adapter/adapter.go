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
	"fmt"
	"net/http"
	"strings"

	"knative.dev/eventing/pkg/adapter"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/pkg/source"

	"context"

	kafkabinding "knative.dev/eventing-contrib/kafka"
	"knative.dev/eventing-contrib/kafka/common/pkg/kafka"

	"github.com/Shopify/sarama"
	"go.uber.org/zap"
	"knative.dev/pkg/logging"
)

const (
	resourceGroup = "kafkasources.sources.knative.dev"
)

type adapterConfig struct {
	adapter.EnvConfig

	Topics        []string `envconfig:"KAFKA_TOPICS" required:"true"`
	ConsumerGroup string   `envconfig:"KAFKA_CONSUMER_GROUP" required:"true"`
	Name          string   `envconfig:"NAME" required:"true"`
	KeyType       string   `envconfig:"KEY_TYPE" required:"false"`
}

func NewEnvConfig() adapter.EnvConfigAccessor {
	return &adapterConfig{}
}

type Adapter struct {
	config            *adapterConfig
	client            sarama.Client
	httpMessageSender *kncloudevents.HttpMessageSender
	reporter          source.StatsReporter
	logger            *zap.Logger
	keyTypeMapper     func([]byte) interface{}
}

var _ adapter.MessageAdapter = (*Adapter)(nil)
var _ adapter.MessageAdapterConstructor = NewAdapter

func NewAdapter(ctx context.Context, processed adapter.EnvConfigAccessor, httpMessageSender *kncloudevents.HttpMessageSender, reporter source.StatsReporter) adapter.MessageAdapter {
	logger := logging.FromContext(ctx).Desugar()
	config := processed.(*adapterConfig)

	client, err := kafkabinding.NewConsumer(context.Background())
	if err != nil {
		panic(err)
	}

	return &Adapter{
		config:            config,
		client:            client,
		httpMessageSender: httpMessageSender,
		reporter:          reporter,
		logger:            logger,
		keyTypeMapper:     getKeyTypeMapper(config.KeyType),
	}
}

// --------------------------------------------------------------------

func (a *Adapter) Start(stopCh <-chan struct{}) error {
	a.logger.Info("Starting with config: ",
		zap.String("Topics", strings.Join(a.config.Topics, ",")),
		zap.String("ConsumerGroup", a.config.ConsumerGroup),
		zap.String("SinkURI", a.config.SinkURI),
		zap.String("Name", a.config.Name),
		zap.String("Namespace", a.config.Namespace),
	)

	defer func() { _ = a.client.Close() }()

	// init consumer group
	consumerGroupFactory := kafka.NewConsumerGroupFactory(a.client)
	group, err := consumerGroupFactory.StartConsumerGroup(a.config.ConsumerGroup, a.config.Topics, a.logger, a)
	if err != nil {
		panic(err)
	}
	defer func() { _ = group.Close() }()

	// Track errors
	go func() {
		for err := range group.Errors() {
			a.logger.Error("An error has occurred while consuming messages occurred: ", zap.Error(err))
		}
	}()

	for {
		select {
		case <-stopCh:
			a.logger.Info("Shutting down...")
			return nil
		}
	}
}

// --------------------------------------------------------------------

func (a *Adapter) Handle(ctx context.Context, msg *sarama.ConsumerMessage) (bool, error) {
	req, err := a.httpMessageSender.NewCloudEventRequest(ctx)
	if err != nil {
		return false, err
	}

	err = a.ConsumerMessageToHttpRequest(ctx, msg, req, a.logger)
	if err != nil {
		return true, err
	}

	res, err := a.httpMessageSender.Send(req)

	if err != nil {
		a.logger.Debug("Error while sending the message", zap.Error(err))
		return false, err // Error while sending, don't commit offset
	}

	if res.StatusCode/100 != 2 {
		a.logger.Debug("Unexpected status code", zap.Int("status code", res.StatusCode))
		return false, fmt.Errorf("%d %s", res.StatusCode, http.StatusText(res.StatusCode))
	}

	reportArgs := &source.ReportArgs{
		Namespace:     a.config.Namespace,
		Name:          a.config.Name,
		ResourceGroup: resourceGroup,
	}

	_ = a.reporter.ReportEventCount(reportArgs, res.StatusCode)
	return true, nil
}
