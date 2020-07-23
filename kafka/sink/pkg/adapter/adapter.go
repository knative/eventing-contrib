/*
Copyright 2018 The Knative Authors

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

package adapter

import (
	"context"
	"fmt"
	"io"
	"strconv"

	"github.com/Shopify/sarama"
	cloudeventskafka "github.com/cloudevents/sdk-go/protocol/kafka_sarama/v2"
	cloudevents "github.com/cloudevents/sdk-go/v2"
	cloudeventshttp "github.com/cloudevents/sdk-go/v2/protocol/http"
	"go.uber.org/zap"
	"knative.dev/eventing/pkg/adapter/v2"
	"knative.dev/pkg/logging"
)

type envConfig struct {
	adapter.EnvConfig

	// Port on which to listen for cloudevents
	Port string `envconfig:"PORT" default:"8080"`

	// KafkaServer URL to connect to the Kafka server.
	KafkaServer string `envconfig:"KAFKA_SERVER" required:"true"`

	// Topic to publish cloudevents to.
	Topic string `envconfig:"KAFKA_TOPIC" required:"true"`
}

// NewEnvConfig function reads env variables defined in envConfig structure and
// returns accessor interface
func NewEnvConfig() adapter.EnvConfigAccessor {
	return &envConfig{}
}

// kafkaSinkAdapter acts as Sink that sends incoming CloudEvents to Kafka topic
type kafkaSinkAdapter struct {
	logger *zap.SugaredLogger
	client cloudevents.Client
	//source string

	port        string
	kafkaServer string
	topic       string
}

// NewAdapter returns the instance of gitHubReceiveAdapter that implements adapter.Adapter interface
func NewAdapter(ctx context.Context, processed adapter.EnvConfigAccessor, ceClient cloudevents.Client) adapter.Adapter {
	logger := logging.FromContext(ctx)
	env := processed.(*envConfig)

	return &kafkaSinkAdapter{
		logger:      logger,
		client:      ceClient,
		port:        env.Port,
		kafkaServer: env.KafkaServer,
		topic:       env.Topic,
	}
}

func (a *kafkaSinkAdapter) Start(ctx context.Context) error {
	saramaConfig := sarama.NewConfig()
	saramaConfig.Version = sarama.V2_0_0_0

	//done := make(chan bool, 1)

	a.logger.Info("Using HTTP PORT=%d", a.port)
	port, err := strconv.Atoi(a.port)
	if err != nil {
		return fmt.Errorf("failed to parse HTTP port %s: %v", a.port, err)
	}

	httpProtocol, err := cloudeventshttp.New(cloudeventshttp.WithPort(port))
	if err != nil {
		return fmt.Errorf("failed to create HTTP protocol: %v", err)
	}

	a.logger.Info("Sinking from HTTP to KAFKA_SERVER=%s KAFKA_TOPIC=%s", a.kafkaServer, a.topic)
	kafkaProtocol, err := cloudeventskafka.NewSender([]string{a.kafkaServer}, saramaConfig, a.topic)
	if err != nil {
		return fmt.Errorf("failed to create Kafka protcol: %v", err)
	}

	defer kafkaProtocol.Close(ctx)

	// Pipe all messages incoming from httpProtocol to kafkaProtocol
	go func() {
		for {
			a.logger.Info("Ready to receive")
			// Blocking call to wait for new messages from httpProtocol
			message, err := httpProtocol.Receive(ctx)
			a.logger.Info("Received message")
			if err != nil {
				if err == io.EOF {
					return // Context closed and/or receiver closed
				}
				a.logger.Info("Error while receiving a message: %s", err.Error())
			}
			a.logger.Info("Sending message to Kafka")
			err = kafkaProtocol.Send(ctx, message)
			if err != nil {
				a.logger.Info("Error while forwarding the message: %s", err.Error())
			}
		}
	}()

	// Start the HTTP Server invoking OpenInbound()
	go func() {
		if err := httpProtocol.OpenInbound(ctx); err != nil {
			a.logger.Info("failed to StartHTTPReceiver, %v", err)
		}
	}()

	a.logger.Infof("Server is ready to handle requests.")

	<-ctx.Done()
	a.logger.Infof("Server stopped")
	return nil
}
