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
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"knative.dev/eventing/pkg/adapter"
	"knative.dev/pkg/source"

	"knative.dev/eventing-contrib/kafka/common/pkg/kafka"

	"context"

	"github.com/Shopify/sarama"
	cloudevents "github.com/cloudevents/sdk-go"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	"go.uber.org/zap"
	sourcesv1alpha1 "knative.dev/eventing-contrib/kafka/source/pkg/apis/sources/v1alpha1"
	"knative.dev/pkg/logging"
)

const (
	resourceGroup = "kafkasources.sources.eventing.knative.dev"
)

type AdapterSASL struct {
	Enable   bool   `envconfig:"KAFKA_NET_SASL_ENABLE" required:"false"`
	User     string `envconfig:"KAFKA_NET_SASL_USER" required:"false"`
	Password string `envconfig:"KAFKA_NET_SASL_PASSWORD" required:"false"`
}

type AdapterTLS struct {
	Enable bool   `envconfig:"KAFKA_NET_TLS_ENABLE" required:"false"`
	Cert   string `envconfig:"KAFKA_NET_TLS_CERT" required:"false"`
	Key    string `envconfig:"KAFKA_NET_TLS_KEY" required:"false"`
	CACert string `envconfig:"KAFKA_NET_TLS_CA_CERT" required:"false"`
}

type AdapterNet struct {
	SASL AdapterSASL
	TLS  AdapterTLS
}

type adapterConfig struct {
	adapter.EnvConfig

	BootstrapServers string `envconfig:"KAFKA_BOOTSTRAP_SERVERS" required:"true"`
	Topics           string `envconfig:"KAFKA_TOPICS" required:"true"`
	ConsumerGroup    string `envconfig:"KAFKA_CONSUMER_GROUP" required:"true"`
	Name             string `envconfig:"NAME" required:"true"`
	Net              AdapterNet
}

func NewEnvConfig() adapter.EnvConfigAccessor {
	return &adapterConfig{}
}

type Adapter struct {
	config     *adapterConfig
	ceClient   client.Client
	reporter   source.StatsReporter
	logger     *zap.Logger
	eventsPool sync.Pool
}

func NewAdapter(ctx context.Context, processed adapter.EnvConfigAccessor, ceClient client.Client, reporter source.StatsReporter) adapter.Adapter {
	logger := logging.FromContext(ctx).Desugar()
	config := processed.(*adapterConfig)

	// Create the events pool
	eventsPool := sync.Pool{}

	return &Adapter{
		config:     config,
		ceClient:   ceClient,
		reporter:   reporter,
		logger:     logger,
		eventsPool: eventsPool,
	}
}

// --------------------------------------------------------------------

func (a *Adapter) Start(stopCh <-chan struct{}) error {
	a.logger.Info("Starting with config: ",
		zap.String("BootstrapServers", a.config.BootstrapServers),
		zap.String("Topics", a.config.Topics),
		zap.String("ConsumerGroup", a.config.ConsumerGroup),
		zap.String("SinkURI", a.config.SinkURI),
		zap.String("Name", a.config.Name),
		zap.String("Namespace", a.config.Namespace),
		zap.Bool("SASL", a.config.Net.SASL.Enable),
		zap.Bool("TLS", a.config.Net.TLS.Enable))

	kafkaConfig := sarama.NewConfig()
	kafkaConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
	kafkaConfig.Version = sarama.V2_0_0_0
	kafkaConfig.Consumer.Return.Errors = true
	kafkaConfig.Net.SASL.Enable = a.config.Net.SASL.Enable
	kafkaConfig.Net.SASL.User = a.config.Net.SASL.User
	kafkaConfig.Net.SASL.Password = a.config.Net.SASL.Password
	kafkaConfig.Net.TLS.Enable = a.config.Net.TLS.Enable

	if a.config.Net.TLS.Enable {
		tlsConfig, err := newTLSConfig(a.config.Net.TLS.Cert, a.config.Net.TLS.Key, a.config.Net.TLS.CACert)
		if err != nil {
			panic(err)
		}
		kafkaConfig.Net.TLS.Config = tlsConfig
	}

	// Start with a ceClient
	client, err := sarama.NewClient(strings.Split(a.config.BootstrapServers, ","), kafkaConfig)
	if err != nil {
		panic(err)
	}
	defer func() { _ = client.Close() }()

	// init consumer group
	consumerGroupFactory := kafka.NewConsumerGroupFactory(client)
	group, err := consumerGroupFactory.StartConsumerGroup(a.config.ConsumerGroup, strings.Split(a.config.Topics, ","), a.logger, a)
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
	if !json.Valid(msg.Value) {
		return true, nil // Message is malformed, commit the offset so it won't be reprocessed
	}

	var event *cloudevents.Event
	if poolRes := a.eventsPool.Get(); poolRes != nil {
		event = poolRes.(*cloudevents.Event)
	} else {
		event = &cloudevents.Event{}
		event.SetSpecVersion(cloudevents.VersionV1)
	}

	event.SetID(fmt.Sprintf("partition:%s/offset:%s", strconv.Itoa(int(msg.Partition)), strconv.FormatInt(msg.Offset, 10)))
	event.SetTime(msg.Timestamp)
	event.SetType(sourcesv1alpha1.KafkaEventType)
	event.SetSource(sourcesv1alpha1.KafkaEventSource(a.config.Namespace, a.config.Name, msg.Topic))
	event.SetDataContentType(cloudevents.ApplicationJSON)
	event.SetExtension("key", string(msg.Key))
	err := event.SetData(msg.Value)

	if err != nil {
		return true, nil // Message is malformed, commit the offset so it won't be reprocessed
	}

	// Check before writing log since event.String() allocates and uses a lot of time
	if ce := a.logger.Check(zap.DebugLevel, "debugging"); ce != nil {
		a.logger.Debug("Sending cloud event", zap.String("event", event.String()))
	}

	rctx, _, err := a.ceClient.Send(ctx, *event)

	a.eventsPool.Put(event)

	if err != nil {
		return false, err // Error while sending, don't commit offset
	}

	reportArgs := &source.ReportArgs{
		Namespace:     a.config.Namespace,
		EventSource:   event.Source(),
		EventType:     event.Type(),
		Name:          a.config.Name,
		ResourceGroup: resourceGroup,
	}

	_ = a.reporter.ReportEventCount(reportArgs, cloudevents.HTTPTransportContextFrom(rctx).StatusCode)
	return true, nil
}

// newTLSConfig returns a *tls.Config using the given ceClient cert, ceClient key,
// and CA certificate. If none are appropriate, a nil *tls.Config is returned.
func newTLSConfig(clientCert, clientKey, caCert string) (*tls.Config, error) {
	valid := false

	config := &tls.Config{}

	if clientCert != "" && clientKey != "" {
		cert, err := tls.X509KeyPair([]byte(clientCert), []byte(clientKey))
		if err != nil {
			return nil, err
		}
		config.Certificates = []tls.Certificate{cert}
		config.BuildNameToCertificate()
		valid = true
	}

	if caCert != "" {
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM([]byte(caCert))
		config.RootCAs = caCertPool
		// The CN of Heroku Kafka certs do not match the hostname of the
		// broker, but Go's default TLS behavior requires that they do.
		config.VerifyPeerCertificate = verifyCertSkipHostname(caCertPool)
		config.InsecureSkipVerify = true
		valid = true
	}

	if !valid {
		config = nil
	}

	return config, nil
}

// verifyCertSkipHostname verifies certificates in the same way that the
// default TLS handshake does, except it skips hostname verification. It must
// be used with InsecureSkipVerify.
func verifyCertSkipHostname(roots *x509.CertPool) func([][]byte, [][]*x509.Certificate) error {
	return func(certs [][]byte, _ [][]*x509.Certificate) error {
		opts := x509.VerifyOptions{
			Roots:         roots,
			CurrentTime:   time.Now(),
			Intermediates: x509.NewCertPool(),
		}

		leaf, err := x509.ParseCertificate(certs[0])
		if err != nil {
			return err
		}

		for _, asn1Data := range certs[1:] {
			cert, err := x509.ParseCertificate(asn1Data)
			if err != nil {
				return err
			}

			opts.Intermediates.AddCert(cert)
		}

		_, err = leaf.Verify(opts)
		return err
	}
}
