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
	"time"

	"knative.dev/eventing-contrib/kafka/common/pkg/kafka"

	"context"

	"github.com/Shopify/sarama"
	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	"go.uber.org/zap"
	sourcesv1alpha1 "knative.dev/eventing-contrib/kafka/source/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing-contrib/pkg/kncloudevents"
	"knative.dev/pkg/logging"
)

type AdapterSASL struct {
	Enable   bool
	User     string
	Password string
}

type AdapterTLS struct {
	Enable bool
	Cert   string
	Key    string
	CACert string
}

type AdapterNet struct {
	SASL AdapterSASL
	TLS  AdapterTLS
}

type Adapter struct {
	BootstrapServers string
	Topics           string
	ConsumerGroup    string
	Net              AdapterNet
	SinkURI          string
	Name             string
	Namespace        string
	ceClient         client.Client
	logger           *zap.Logger
}

// --------------------------------------------------------------------

func (a *Adapter) Handle(ctx context.Context, msg *sarama.ConsumerMessage) (bool, error) {
	jsonPayload, err := a.jsonEncode(msg.Value)

	if err != nil {
		return true, nil // Message is malformed, commit the offset so it won't be reprocessed
	}

	event := cloudevents.New(cloudevents.CloudEventsVersionV02)
	event.SetID(fmt.Sprintf("partition:%s/offset:%s", strconv.Itoa(int(msg.Partition)), strconv.FormatInt(msg.Offset, 10)))
	event.SetTime(msg.Timestamp)
	event.SetType(sourcesv1alpha1.KafkaEventType)
	event.SetSource(sourcesv1alpha1.KafkaEventSource(a.Namespace, a.Name, msg.Topic))
	event.SetDataContentType(*cloudevents.StringOfApplicationJSON())
	event.SetExtension("key", string(msg.Key))
	err = event.SetData(jsonPayload)

	if err != nil {
		return true, nil // Message is malformed, commit the offset so it won't be reprocessed
	}

	a.logger.Debug("Sending cloud event", zap.String("event", event.String()))

	_, _, err = a.ceClient.Send(ctx, event)

	if err != nil {
		return false, err // Error while sending, don't commit offset
	}

	return true, nil
}

// --------------------------------------------------------------------

func (a *Adapter) Start(ctx context.Context, stopCh <-chan struct{}) error {
	logger := logging.FromContext(ctx)
	a.logger = logger.Desugar()

	logger.Infow("Starting with config: ",
		zap.String("BootstrapServers", a.BootstrapServers),
		zap.String("Topics", a.Topics),
		zap.String("ConsumerGroup", a.ConsumerGroup),
		zap.String("SinkURI", a.SinkURI),
		zap.String("Name", a.Name),
		zap.String("Namespace", a.Namespace),
		zap.Bool("SASL", a.Net.SASL.Enable),
		zap.Bool("TLS", a.Net.TLS.Enable))

	kafkaConfig := sarama.NewConfig()
	kafkaConfig.Consumer.Offsets.Initial = sarama.OffsetOldest
	kafkaConfig.Version = sarama.V2_0_0_0
	kafkaConfig.Consumer.Return.Errors = true
	kafkaConfig.Net.SASL.Enable = a.Net.SASL.Enable
	kafkaConfig.Net.SASL.User = a.Net.SASL.User
	kafkaConfig.Net.SASL.Password = a.Net.SASL.Password
	kafkaConfig.Net.TLS.Enable = a.Net.TLS.Enable

	if a.Net.TLS.Enable {
		tlsConfig, err := newTLSConfig(a.Net.TLS.Cert, a.Net.TLS.Key, a.Net.TLS.CACert)
		if err != nil {
			panic(err)
		}
		kafkaConfig.Net.TLS.Config = tlsConfig
	}

	// Start the cloud events client
	if a.ceClient == nil {
		var err error
		if a.ceClient, err = kncloudevents.NewDefaultClient(a.SinkURI); err != nil {
			return err
		}
	}

	// Start with a ceClient
	client, err := sarama.NewClient(strings.Split(a.BootstrapServers, ","), kafkaConfig)
	if err != nil {
		panic(err)
	}
	defer func() { _ = client.Close() }()

	// init consumer group
	consumerGroupFactory := kafka.NewConsumerGroupFactory(client)
	group, err := consumerGroupFactory.StartConsumerGroup(a.ConsumerGroup, strings.Split(a.Topics, ","), logger.Desugar(), a)
	if err != nil {
		panic(err)
	}
	defer func() { _ = group.Close() }()

	// Track errors
	go func() {
		for err := range group.Errors() {
			logger.Error("An error has occurred while consuming messages occurred: ", zap.Error(err))
		}
	}()

	for {
		select {
		case <-stopCh:
			logger.Infow("Shutting down...")
			return nil
		}
	}
}

func (a *Adapter) jsonEncode(value []byte) (map[string]interface{}, error) {
	var payload map[string]interface{}

	if err := json.Unmarshal(value, &payload); err != nil {
		return nil, err
	} else {
		return payload, nil
	}
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
