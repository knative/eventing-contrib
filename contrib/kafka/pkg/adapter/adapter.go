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

	"github.com/Shopify/sarama"
	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	sourcesv1alpha1 "github.com/knative/eventing-sources/contrib/kafka/pkg/apis/sources/v1alpha1"
	"github.com/knative/eventing-sources/pkg/kncloudevents"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	"golang.org/x/net/context"
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
	client           client.Client
}

// --------------------------------------------------------------------

// ConsumerGroupHandler functions to define message consume and related logic.
func (a *Adapter) Setup(_ sarama.ConsumerGroupSession) error {
	if a.client == nil {
		var err error
		if a.client, err = kncloudevents.NewDefaultClient(a.SinkURI); err != nil {
			return err
		}
	}
	return nil
}
func (a *Adapter) Cleanup(_ sarama.ConsumerGroupSession) error { return nil }
func (a *Adapter) ConsumeClaim(sess sarama.ConsumerGroupSession, claim sarama.ConsumerGroupClaim) error {

	logger := logging.FromContext(context.TODO())

	for msg := range claim.Messages() {
		logger.Debug("Received: ", zap.String("topic:", msg.Topic),
			zap.Int32("partition:", msg.Partition),
			zap.Int64("offset:", msg.Offset))

		// send and mark message if post was successful
		if err := a.postMessage(context.TODO(), msg); err == nil {
			sess.MarkMessage(msg, "")
			logger.Debug("Successfully sent event to sink")
		} else {
			logger.Error("Sending event to sink failed: ", zap.Error(err))
		}
	}
	return nil
}

// --------------------------------------------------------------------

func (a *Adapter) Start(ctx context.Context, stopCh <-chan struct{}) error {
	logger := logging.FromContext(ctx)

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

	// Start with a client
	client, err := sarama.NewClient(strings.Split(a.BootstrapServers, ","), kafkaConfig)
	if err != nil {
		panic(err)
	}
	defer func() { _ = client.Close() }()

	// init consumer group
	group, err := sarama.NewConsumerGroupFromClient(a.ConsumerGroup, client)
	if err != nil {
		panic(err)
	}
	defer func() { _ = group.Close() }()

	// Track errors
	go func() {
		for err := range group.Errors() {
			logger.Error("A consumer group error occurred: ", zap.Error(err))
		}
	}()

	// Handle session
	go func() {
		for {
			if err := group.Consume(ctx, strings.Split(a.Topics, ","), a); err != nil {
				panic(err)
			}
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

func (a *Adapter) postMessage(ctx context.Context, msg *sarama.ConsumerMessage) error {

	event := cloudevents.New(cloudevents.CloudEventsVersionV02)
	event.SetID(fmt.Sprintf("partition:%s/offset:%s", strconv.Itoa(int(msg.Partition)), strconv.FormatInt(msg.Offset, 10)))
	event.SetTime(msg.Timestamp)
	event.SetType(sourcesv1alpha1.KafkaEventType)
	event.SetSource(sourcesv1alpha1.KafkaEventSource(a.Namespace, a.Name, msg.Topic))
	event.SetDataContentType(*cloudevents.StringOfApplicationJSON())
	event.SetExtension("key", string(msg.Key))
	event.SetData(a.jsonEncode(ctx, msg.Value))

	_, err := a.client.Send(ctx, event)
	return err
}

func (a *Adapter) jsonEncode(ctx context.Context, value []byte) interface{} {
	var payload map[string]interface{}

	logger := logging.FromContext(ctx)

	if err := json.Unmarshal(value, &payload); err != nil {
		logger.Info("Error unmarshalling JSON: ", zap.Error(err))
		return value
	} else {
		return payload
	}
}

// newTLSConfig returns a *tls.Config using the given client cert, client key,
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
