/*
Copyright 2020 The Knative Authors

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
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"time"

	"knative.dev/eventing-contrib/kafka/channel/pkg/utils"

	"github.com/Shopify/sarama"
	"github.com/kelseyhightower/envconfig"
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

type envConfig struct {
	BootstrapServers []string `envconfig:"KAFKA_BOOTSTRAP_SERVERS" required:"true"`
	Net              AdapterNet
}

// NewConfig extracts the Kafka configuration from the environment.
func NewConfig(ctx context.Context) ([]string, *sarama.Config, error) {
	cfg := sarama.NewConfig()
	cfg.Version = sarama.V2_0_0_0
	cfg.Consumer.Return.Errors = true

	var env envConfig
	if err := envconfig.Process("", &env); err != nil {
		return nil, nil, err
	}

	if env.Net.SASL.Enable {
		cfg.Net.SASL.Enable = true
		cfg.Net.SASL.User = env.Net.SASL.User
		cfg.Net.SASL.Password = env.Net.SASL.Password
	}

	if env.Net.TLS.Enable {
		cfg.Net.TLS.Enable = true
		tlsConfig, err := NewTLSConfig(env.Net.TLS.Cert, env.Net.TLS.Key, env.Net.TLS.CACert)
		if err != nil {
			return nil, nil, err
		}
		cfg.Net.TLS.Config = tlsConfig
	}

	return env.BootstrapServers, cfg, nil
}

// NewProducer is a helper method for constructing a client for producing kafka methods.
func NewProducer(ctx context.Context) (sarama.Client, error) {
	bs, cfg, err := NewConfig(ctx)
	if err != nil {
		return nil, err
	}

	cfg.Producer.Return.Successes = true
	cfg.Producer.Return.Errors = true

	return sarama.NewClient(bs, cfg)
}

// NewTLSConfig returns a *tls.Config using the given ceClient cert, ceClient key,
// and CA certificate. If none are appropriate, a nil *tls.Config is returned.
func NewTLSConfig(clientCert, clientKey, caCert string) (*tls.Config, error) {
	valid := false

	config := &tls.Config{}

	if clientCert != "" && clientKey != "" {
		cert, err := tls.X509KeyPair([]byte(clientCert), []byte(clientKey))
		if err != nil {
			return nil, err
		}
		config.Certificates = []tls.Certificate{cert}
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

func MakeAdminClient(clientID string, kafkaAuthCfg *utils.KafkaAuthConfig, bootstrapServers []string) (sarama.ClusterAdmin, error) {
	saramaConf := sarama.NewConfig()
	saramaConf.Version = sarama.V2_0_0_0
	saramaConf.ClientID = clientID

	err := UpdateSaramaConfigWithKafkaAuthConfig(saramaConf, kafkaAuthCfg)
	if err != nil {
		return nil, fmt.Errorf("Error creating the Admin Client: %w", err)
	}

	return sarama.NewClusterAdmin(bootstrapServers, saramaConf)
}

func UpdateSaramaConfigWithKafkaAuthConfig(saramaConf *sarama.Config, kafkaAuthCfg *utils.KafkaAuthConfig) error {
	if kafkaAuthCfg != nil {
		// tls
		if kafkaAuthCfg.TLS != nil {
			saramaConf.Net.TLS.Enable = true
			tlsConfig, err := NewTLSConfig(kafkaAuthCfg.TLS.Usercert, kafkaAuthCfg.TLS.Userkey, kafkaAuthCfg.TLS.Cacert)
			if err != nil {
				return fmt.Errorf("Error creating TLS config: %w", err)
			}
			saramaConf.Net.TLS.Config = tlsConfig
		}
		// SASL
		if kafkaAuthCfg.SASL != nil {
			saramaConf.Net.SASL.Enable = true
			saramaConf.Net.SASL.Handshake = true

			// if SaslType is not provided we are defaulting to PLAIN
			saramaConf.Net.SASL.Mechanism = sarama.SASLTypePlaintext

			if kafkaAuthCfg.SASL.SaslType == sarama.SASLTypeSCRAMSHA256 {
				saramaConf.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA256} }
				saramaConf.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA256
			}

			if kafkaAuthCfg.SASL.SaslType == sarama.SASLTypeSCRAMSHA512 {
				saramaConf.Net.SASL.SCRAMClientGeneratorFunc = func() sarama.SCRAMClient { return &XDGSCRAMClient{HashGeneratorFcn: SHA512} }
				saramaConf.Net.SASL.Mechanism = sarama.SASLTypeSCRAMSHA512
			}
			saramaConf.Net.SASL.User = kafkaAuthCfg.SASL.User
			saramaConf.Net.SASL.Password = kafkaAuthCfg.SASL.Password
		}
	}
	return nil
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
