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

package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/cloudevents/sdk-go/v2/protocol/http"
	nethttp "net/http"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	"github.com/cloudevents/sdk-go/v2/binding"
	"go.uber.org/zap"
	"github.com/sbcd90/wabbit"
	"github.com/sbcd90/wabbit/amqp"
	"github.com/sbcd90/wabbit/amqptest"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/source"
	"knative.dev/eventing/pkg/adapter/v2"
	"strings"
	sourcesv1alpha1 "knative.dev/eventing-contrib/rabbitmq/source/pkg/apis/sources/v1alpha1"
)

const (
	resourceGroup 	  = "rabbitmqsources.sources.knative.dev"
)

type ExchangeConfig struct {
	Name        string `envconfig:"RABBITMQ_EXCHANGE_CONFIG_NAME" required:"false"`
	TypeOf      string `envconfig:"RABBITMQ_EXCHANGE_CONFIG_TYPE" required:"true"`
	Durable     bool   `envconfig:"RABBITMQ_EXCHANGE_CONFIG_DURABLE" required:"false"`
	AutoDeleted bool   `envconfig:"RABBITMQ_EXCHANGE_CONFIG_AUTO_DELETED" required:"false"`
	Internal    bool   `envconfig:"RABBITMQ_EXCHANGE_CONFIG_INTERNAL" required:"false"`
	NoWait      bool   `envconfig:"RABBITMQ_EXCHANGE_CONFIG_NOWAIT" required:"false"`
}

type ChannelConfig struct {
	PrefetchCount int
	GlobalQos     bool
}

type QueueConfig struct {
	Name             string `envconfig:"RABBITMQ_QUEUE_CONFIG_NAME" required:"false"`
	RoutingKey       string `envconfig:"RABBITMQ_ROUTING_KEY" required:"true"`
	Durable          bool   `envconfig:"RABBITMQ_QUEUE_CONFIG_DURABLE" required:"false"`
	DeleteWhenUnused bool   `envconfig:"RABBITMQ_QUEUE_CONFIG_AUTO_DELETED" required:"false"`
	Exclusive        bool   `envconfig:"RABBITMQ_QUEUE_CONFIG_EXCLUSIVE" required:"false"`
	NoWait           bool   `envconfig:"RABBITMQ_QUEUE_CONFIG_NOWAIT" required:"false"`
}

type adapterConfig struct {
	adapter.EnvConfig

	Brokers        string `envconfig:"RABBITMQ_BROKERS" required:"true"`
	Topic          string `envconfig:"RABBITMQ_TOPIC" required:"true"`
	User           string `envconfig:"RABBITMQ_USER" required:"false"`
	Password       string `envconfig:"RABBITMQ_PASSWORD" required:"false"`
	ChannelConfig  ChannelConfig
	ExchangeConfig ExchangeConfig
	QueueConfig    QueueConfig
}

func NewEnvConfig() adapter.EnvConfigAccessor {
	return &adapterConfig{}
}

type Adapter struct {
	config 			  *adapterConfig
	httpMessageSender *kncloudevents.HttpMessageSender
	reporter 		  source.StatsReporter
	logger 			  *zap.Logger
	context			  context.Context
}

var _ adapter.MessageAdapter = (*Adapter)(nil)
var _ adapter.MessageAdapterConstructor = NewAdapter

func NewAdapter(ctx context.Context, processed adapter.EnvConfigAccessor, httpMessageSender *kncloudevents.HttpMessageSender, reporter source.StatsReporter) adapter.MessageAdapter {
	logger := logging.FromContext(ctx).Desugar()
	config := processed.(*adapterConfig)

	return &Adapter{
		config: 		   config,
		httpMessageSender: httpMessageSender,
		reporter:		   reporter,
		logger: 		   logger,
		context: 		   ctx,
	}
}

func (a *Adapter) CreateConn(User string, Password string, logger *zap.Logger) (*amqp.Conn, error) {
	if User != "" && Password != "" {
		a.config.Brokers = fmt.Sprintf("amqp://%s:%s@%s", a.config.User, a.config.Password, a.config.Brokers)
	}
	conn, err := amqp.Dial(a.config.Brokers)
	if err != nil {
		logger.Error(err.Error())
	}
	return conn, err
}

func (a *Adapter) CreateChannel(conn *amqp.Conn, connTest *amqptest.Conn,
	logger *zap.Logger) (wabbit.Channel, error) {
	var ch wabbit.Channel
	var err error

	if conn != nil {
		ch, err = conn.Channel()
	} else {
		ch, err = connTest.Channel()
	}
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}

	err = ch.Qos(a.config.ChannelConfig.PrefetchCount,
		0,
		a.config.ChannelConfig.GlobalQos)

	return ch, err
}

func (a *Adapter) Start(stopCh <-chan struct{}) error {
	logger := a.logger

	logger.Info("Starting with config: ", zap.Any("adapter", a))

	conn, err := a.CreateConn(a.config.User, a.config.Password, logger)
	if err == nil {
		defer conn.Close()
	}

	ch, err := a.CreateChannel(conn, nil, logger)
	if err == nil {
		defer ch.Close()
	}

	queue, err := a.StartAmqpClient(&ch)
	if err != nil {
		logger.Error(err.Error())
	}

	return a.PollForMessages(&ch, queue, stopCh)
}

func (a *Adapter) StartAmqpClient(ch *wabbit.Channel) (*wabbit.Queue, error) {
	logger := a.logger
	exchangeConfig := fillDefaultValuesForExchangeConfig(&a.config.ExchangeConfig, a.config.Topic)

	err := (*ch).ExchangeDeclare(
		exchangeConfig.Name,
		exchangeConfig.TypeOf,
		wabbit.Option{
			"durable":  exchangeConfig.Durable,
			"delete":   exchangeConfig.AutoDeleted,
			"internal": exchangeConfig.Internal,
			"noWait":   exchangeConfig.NoWait,
		},)
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}

	queue, err := (*ch).QueueDeclare(
		a.config.QueueConfig.Name,
		wabbit.Option{
			"durable":   a.config.QueueConfig.Durable,
			"delete":    a.config.QueueConfig.DeleteWhenUnused,
			"exclusive": a.config.QueueConfig.Exclusive,
			"noWait":    a.config.QueueConfig.NoWait,
		},)
	if err != nil {
		logger.Error(err.Error())
		return nil, err
	}

	if a.config.ExchangeConfig.TypeOf != "fanout" {
		routingKeys := strings.Split(a.config.QueueConfig.RoutingKey, ",")

		for _, routingKey := range routingKeys {
			err = (*ch).QueueBind(
				queue.Name(),
				routingKey,
				exchangeConfig.Name,
				wabbit.Option{
					"noWait": a.config.QueueConfig.NoWait,
				},)
			if err != nil {
				logger.Error(err.Error())
				return nil, err
			}
		}
	} else {
		err = (*ch).QueueBind(
			queue.Name(),
			"",
			exchangeConfig.Name,
			wabbit.Option{
				"noWait": a.config.QueueConfig.NoWait,
			})
		if err != nil {
			logger.Error(err.Error())
			return nil, err
		}
	}

	return &queue, nil
}

func (a *Adapter) ConsumeMessages(channel *wabbit.Channel,
	queue *wabbit.Queue, logger *zap.Logger) (<-chan wabbit.Delivery, error) {
	msgs, err := (*channel).Consume(
		(*queue).Name(),
		"",
		wabbit.Option{
			"noAck":     false,
			"exclusive": a.config.QueueConfig.Exclusive,
			"noLocal":   false,
			"noWait":    a.config.QueueConfig.NoWait,
		},)

	if err != nil {
		logger.Error(err.Error())
	}
	return msgs, err
}

func (a *Adapter) PollForMessages(channel *wabbit.Channel,
	queue *wabbit.Queue, stopCh <-chan struct{}) error {
	logger := a.logger

	msgs, _ := a.ConsumeMessages(channel, queue, logger)

	for {
		select {
		case msg, ok := <-msgs:
			if ok {
				logger.Info("Received: ", zap.Any("value", string(msg.Body())))

				go func(message *wabbit.Delivery) {
					if err := a.postMessage(*message); err == nil {
						logger.Info("Successfully sent event to sink")
						err = (*channel).Ack((*message).DeliveryTag(), true)
						if err != nil {
							logger.Error("Sending Ack failed with Delivery Tag")
						}
					} else {
						logger.Error("Sending event to sink failed: ", zap.Error(err))
						err = (*channel).Nack((*message).DeliveryTag(), true, true)
						if err != nil {
							logger.Error("Sending Nack failed with Delivery Tag")
						}
					}
				}(&msg)
			} else {
				return nil
			}
		case <-stopCh:
			logger.Info("Shutting down...")
			return nil
		}
	}
}

func (a *Adapter) postMessage(msg wabbit.Delivery) error {
	a.logger.Info("url ->" + a.httpMessageSender.Target)
	req, err := a.httpMessageSender.NewCloudEventRequest(a.context)
	if err != nil {
		return err
	}

	event := cloudevents.NewEvent()

	event.SetID(msg.MessageId())
	event.SetTime(msg.Timestamp())
	event.SetType(sourcesv1alpha1.RabbitmqEventType)
	event.SetSource(sourcesv1alpha1.RabbitmqEventSource(a.config.Namespace, a.config.Name, a.config.Topic))
	event.SetSubject(msg.MessageId())
	event.SetExtension("key", msg.MessageId())

	err = event.SetData(*cloudevents.StringOfApplicationJSON(), a.JsonEncode(msg.Body()))
	if err != nil {
		return err
	}

	err = http.WriteRequest(a.context, binding.ToMessage(&event), req)
	if err != nil {
		return err
	}

	res, err := a.httpMessageSender.Send(req)

	if err != nil {
		a.logger.Debug("Error while sending the message", zap.Error(err))
		return err
	}

	if res.StatusCode/100 != 2 {
		a.logger.Debug("Unexpected status code", zap.Int("status code", res.StatusCode))
		return fmt.Errorf("%d %s", res.StatusCode, nethttp.StatusText(res.StatusCode))
	}

	reportArgs := &source.ReportArgs{
		Namespace: 	   a.config.Namespace,
		Name:          a.config.Name,
		ResourceGroup: resourceGroup,
	}

	_ = a.reporter.ReportEventCount(reportArgs, res.StatusCode)
	return nil
}

func (a *Adapter) JsonEncode(body []byte) interface{} {
	var payload map[string]interface{}

	logger := a.logger

	if err := json.Unmarshal(body, &payload); err != nil {
		logger.Info("Error unmarshalling JSON: ", zap.Error(err))
		return body
	} else {
		return payload
	}
}

func fillDefaultValuesForExchangeConfig(config *ExchangeConfig, topic string) *ExchangeConfig {
	if config.TypeOf != "topic" {
		if config.Name == "" {
			config.Name = "logs"
		}
	} else {
		config.Name = topic
	}
	return config
}