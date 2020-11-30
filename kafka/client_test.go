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
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/Shopify/sarama"
	"knative.dev/eventing-contrib/kafka/channel/pkg/utils"

	"github.com/stretchr/testify/require"
)

func TestUpdateSaramaConfigWithKafkaAuthConfig(t *testing.T) {

	cert, key := generateCert(t)

	testCases := map[string]struct {
		kafkaAuthCfg  *utils.KafkaAuthConfig
		enabledTLS    bool
		enabledSASL   bool
		salsMechanism string
	}{
		"No Auth": {
			enabledTLS:  false,
			enabledSASL: false,
		},
		"Only SASL-PLAIN Auth": {
			kafkaAuthCfg: &utils.KafkaAuthConfig{
				SASL: &utils.KafkaSaslConfig{
					User:     "my-user",
					Password: "super-secret",
					SaslType: sarama.SASLTypePlaintext,
				},
			},
			enabledTLS:    false,
			enabledSASL:   true,
			salsMechanism: sarama.SASLTypePlaintext,
		},
		"Only SASL-PLAIN Auth (not specified, defaulted)": {
			kafkaAuthCfg: &utils.KafkaAuthConfig{
				SASL: &utils.KafkaSaslConfig{
					User:     "my-user",
					Password: "super-secret",
				},
			},
			enabledTLS:    false,
			enabledSASL:   true,
			salsMechanism: sarama.SASLTypePlaintext,
		},
		"Only SASL-SCRAM-SHA-256 Auth": {
			kafkaAuthCfg: &utils.KafkaAuthConfig{
				SASL: &utils.KafkaSaslConfig{
					User:     "my-user",
					Password: "super-secret",
					SaslType: sarama.SASLTypeSCRAMSHA256,
				},
			},
			enabledTLS:    false,
			enabledSASL:   true,
			salsMechanism: sarama.SASLTypeSCRAMSHA256,
		},
		"Only SASL-SCRAM-SHA-512 Auth": {
			kafkaAuthCfg: &utils.KafkaAuthConfig{
				SASL: &utils.KafkaSaslConfig{
					User:     "my-user",
					Password: "super-secret",
					SaslType: sarama.SASLTypeSCRAMSHA512,
				},
			},
			enabledTLS:    false,
			enabledSASL:   true,
			salsMechanism: sarama.SASLTypeSCRAMSHA512,
		},
		"Only TLS Auth": {
			kafkaAuthCfg: &utils.KafkaAuthConfig{
				TLS: &utils.KafkaTlsConfig{
					Cacert:   cert,
					Usercert: cert,
					Userkey:  key,
				},
			},
			enabledTLS:  true,
			enabledSASL: false,
		},
		"SASL and TLS Auth": {
			kafkaAuthCfg: &utils.KafkaAuthConfig{
				SASL: &utils.KafkaSaslConfig{
					User:     "my-user",
					Password: "super-secret",
					SaslType: sarama.SASLTypeSCRAMSHA512,
				},
				TLS: &utils.KafkaTlsConfig{
					Cacert:   cert,
					Usercert: cert,
					Userkey:  key,
				},
			},
			enabledTLS:    true,
			enabledSASL:   true,
			salsMechanism: sarama.SASLTypeSCRAMSHA512,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {

			// Perform The Test
			config := sarama.NewConfig()
			UpdateSaramaConfigWithKafkaAuthConfig(config, tc.kafkaAuthCfg)

			saslEnabled := config.Net.SASL.Enable
			if saslEnabled != tc.enabledSASL {
				t.Errorf("SASL config is wrong")
			}
			if saslEnabled {
				if tc.salsMechanism != string(config.Net.SASL.Mechanism) {
					t.Errorf("SASL Mechanism is wrong, want: %s vs got %s", tc.salsMechanism, string(config.Net.SASL.Mechanism))
				}
			}

			tlsEnabled := config.Net.TLS.Enable
			if tlsEnabled != tc.enabledTLS {
				t.Errorf("TLS config is wrong")
			}
			if tlsEnabled {
				if config.Net.TLS.Config == nil {
					t.Errorf("TLS config is wrong")
				}
			}
		})
	}
}

func TestNewTLSConfig(t *testing.T) {
	cert, key := generateCert(t)

	for _, tt := range []struct {
		name       string
		cert       string
		key        string
		caCert     string
		wantErr    bool
		wantNil    bool
		wantClient bool
		wantServer bool
	}{{
		name:    "all empty",
		wantNil: true,
	}, {
		name:    "bad input",
		cert:    "x",
		key:     "y",
		caCert:  "z",
		wantErr: true,
	}, {
		name:    "only cert",
		cert:    cert,
		wantNil: true,
	}, {
		name:    "only key",
		key:     key,
		wantNil: true,
	}, {
		name:       "cert and key",
		cert:       cert,
		key:        key,
		wantClient: true,
	}, {
		name:       "only caCert",
		caCert:     cert,
		wantServer: true,
	}, {
		name:       "cert, key, and caCert",
		cert:       cert,
		key:        key,
		caCert:     cert,
		wantClient: true,
		wantServer: true,
	}} {
		t.Run(tt.name, func(t *testing.T) {
			c, err := NewTLSConfig(tt.cert, tt.key, tt.caCert)
			if tt.wantErr {
				if err == nil {
					t.Fatal("wanted error")
				}
				return
			}

			if tt.wantNil {
				if c != nil {
					t.Fatal("wanted non-nil config")
				}
				return
			}

			var wantCertificates int
			if tt.wantClient {
				wantCertificates = 1
			} else {
				wantCertificates = 0
			}
			if got, want := len(c.Certificates), wantCertificates; got != want {
				t.Errorf("got %d Certificates, wanted %d", got, want)
			}

			if tt.wantServer {
				if c.RootCAs == nil {
					t.Error("wanted non-nil RootCAs")
				}

				if c.VerifyPeerCertificate == nil {
					t.Error("wanted non-nil VerifyPeerCertificate")
				}

				if !c.InsecureSkipVerify {
					t.Error("wanted InsecureSkipVerify")
				}
			} else {
				if c.RootCAs != nil {
					t.Error("wanted nil RootCAs")
				}

				if c.VerifyPeerCertificate != nil {
					t.Error("wanted nil VerifyPeerCertificate")
				}

				if c.InsecureSkipVerify {
					t.Error("wanted false InsecureSkipVerify")
				}
			}
		})
	}

}

func TestVerifyCertSkipHostname(t *testing.T) {
	cert, _ := generateCert(t)
	certPem, _ := pem.Decode([]byte(cert))

	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM([]byte(cert))

	v := verifyCertSkipHostname(caCertPool)

	err := v([][]byte{certPem.Bytes}, nil)
	if err != nil {
		t.Fatal(err)
	}

	cert2, _ := generateCert(t)
	cert2Pem, _ := pem.Decode([]byte(cert2))

	err = v([][]byte{cert2Pem.Bytes}, nil)
	// Error expected as we're still verifying with the first cert.
	if err == nil {
		t.Fatal("wanted error")
	}
}

// Lifted from the RSA path of https://golang.org/src/crypto/tls/generate_cert.go.
func generateCert(t *testing.T) (string, string) {
	t.Helper()

	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}

	notBefore := time.Now().Add(-5 * time.Minute)
	notAfter := notBefore.Add(time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)

	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)

	if err != nil {
		t.Fatal(err)
	}

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Acme Co"},
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}

	var certOut bytes.Buffer
	if err := pem.Encode(&certOut, &pem.Block{Type: "CERTIFICATE", Bytes: derBytes}); err != nil {
		t.Fatal(err)
	}

	var keyOut bytes.Buffer
	if err := pem.Encode(&keyOut, &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(priv)}); err != nil {
		t.Fatal(err)
	}

	return certOut.String(), keyOut.String()
}

func TestNewConfig(t *testing.T) {
	// Increasing coverage
	defaultBootstrapServer := "my-cluster-kafka-bootstrap.my-kafka-namespace:9092"
	defaultSASLUser := "secret-user"
	defaultSASLPassword := "super-seekrit-password"
	testCases := map[string]struct {
		env             map[string]string
		enabledTLS      bool
		enabledSASL     bool
		wantErr         bool
		saslMechanism   string
		bootstrapServer string
		saslUser        string
		saslPassword    string
	}{
		"Just bootstrap Server": {
			env: map[string]string{
				"KAFKA_BOOTSTRAP_SERVERS": defaultBootstrapServer,
			},
			bootstrapServer: defaultBootstrapServer,
		},
		"Incorrect bootstrap Server": {
			env: map[string]string{
				"KAFKA_BOOTSTRAP_SERVERS": defaultBootstrapServer,
			},
			bootstrapServer: "ImADoctorNotABootstrapServerJim!",
			wantErr:         true,
		},
		/*
			TODO
			"Multiple bootstrap servers": {
				env: map[string]string{

				},

			},*/
		"No Auth": {
			env: map[string]string{
				"KAFKA_BOOTSTRAP_SERVERS": defaultBootstrapServer,
			},
			enabledTLS:      false,
			enabledSASL:     false,
			bootstrapServer: defaultBootstrapServer,
		},
		"Defaulting to SASL-Plain Auth (none specified)": {
			env: map[string]string{
				"KAFKA_BOOTSTRAP_SERVERS": defaultBootstrapServer,
				"KAFKA_NET_SASL_ENABLE":   "true",
			},
			enabledSASL:     true,
			saslMechanism:   sarama.SASLTypePlaintext,
			bootstrapServer: defaultBootstrapServer,
		},
		"Only SASL-PLAIN Auth": {
			env: map[string]string{
				"KAFKA_BOOTSTRAP_SERVERS": defaultBootstrapServer,
				"KAFKA_NET_SASL_ENABLE":   "true",
				"KAFKA_NET_SASL_USER":     defaultSASLUser,
				"KAFKA_NET_SASL_PASSWORD": defaultSASLPassword,
			},
			enabledSASL:     true,
			saslUser:        defaultSASLUser,
			saslPassword:    defaultSASLPassword,
			saslMechanism:   sarama.SASLTypePlaintext,
			bootstrapServer: defaultBootstrapServer,
		},
		"SASL-PLAIN Auth, Forgot User": {
			env: map[string]string{
				"KAFKA_BOOTSTRAP_SERVERS": defaultBootstrapServer,
				"KAFKA_NET_SASL_ENABLE":   "true",
				"KAFKA_NET_SASL_PASSWORD": defaultSASLPassword,
			},
			enabledSASL:     true,
			wantErr:         true,
			saslUser:        defaultSASLUser,
			saslPassword:    defaultSASLPassword,
			saslMechanism:   sarama.SASLTypePlaintext,
			bootstrapServer: defaultBootstrapServer,
		},
		"SASL-PLAIN Auth, Forgot Password": {
			env: map[string]string{
				"KAFKA_BOOTSTRAP_SERVERS": defaultBootstrapServer,
				"KAFKA_NET_SASL_ENABLE":   "true",
				"KAFKA_NET_SASL_USER":     defaultSASLUser,
			},
			enabledSASL:     true,
			wantErr:         true,
			saslUser:        defaultSASLUser,
			saslPassword:    defaultSASLPassword,
			saslMechanism:   sarama.SASLTypePlaintext,
			bootstrapServer: defaultBootstrapServer,
		},
		"Only SASL-SCRAM-SHA-256 Auth": {
			env: map[string]string{
				"KAFKA_BOOTSTRAP_SERVERS": defaultBootstrapServer,
				"KAFKA_NET_SASL_ENABLE":   "true",
				"KAFKA_NET_SASL_USER":     defaultSASLUser,
				"KAFKA_NET_SASL_PASSWORD": defaultSASLPassword,
				"KAFKA_NET_SASL_TYPE":     sarama.SASLTypeSCRAMSHA256,
			},
			enabledSASL:     true,
			saslUser:        defaultSASLUser,
			saslPassword:    defaultSASLPassword,
			saslMechanism:   sarama.SASLTypeSCRAMSHA256,
			bootstrapServer: defaultBootstrapServer,
		},
		"Only SASL-SCRAM-SHA-512 Auth": {
			env: map[string]string{
				"KAFKA_BOOTSTRAP_SERVERS": defaultBootstrapServer,
				"KAFKA_NET_SASL_ENABLE":   "true",
				"KAFKA_NET_SASL_USER":     defaultSASLUser,
				"KAFKA_NET_SASL_PASSWORD": defaultSASLPassword,
				"KAFKA_NET_SASL_TYPE":     sarama.SASLTypeSCRAMSHA512,
			},
			enabledSASL:     true,
			saslUser:        defaultSASLUser,
			saslPassword:    defaultSASLPassword,
			saslMechanism:   sarama.SASLTypeSCRAMSHA512,
			bootstrapServer: defaultBootstrapServer,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			for k, v := range tc.env {
				_ = os.Setenv(k, v)
			}
			servers, config, err := NewConfig(context.Background())
			if err != nil && tc.wantErr != true {
				t.Fatal(err)
			}
			if servers[0] != tc.bootstrapServer && tc.wantErr != true {
				t.Fatalf("Incorrect bootstrapServers, got: %s vs want: %s", servers[0], tc.bootstrapServer)
			}
			if tc.enabledSASL {
				if tc.saslMechanism != string(config.Net.SASL.Mechanism) {
					t.Fatalf("Incorrect SASL mechanism, got: %s vs want: %s", string(config.Net.SASL.Mechanism), tc.saslMechanism)
				}

				if config.Net.SASL.Enable != true {
					t.Fatal("Incorrect SASL Configuration (not enabled)")
				}

				if config.Net.SASL.User != tc.saslUser && !tc.wantErr {
					t.Fatalf("Incorrect SASL User, got: %s vs want: %s", config.Net.SASL.User, tc.saslUser)
				}
				if config.Net.SASL.Password != tc.saslPassword && !tc.wantErr {
					t.Fatalf("Incorrect SASL Password, got: %s vs want: %s", config.Net.SASL.Password, tc.saslPassword)
				}
			}
			require.NotNil(t, config)
			for k := range tc.env {
				_ = os.Unsetenv(k)
			}
		})
	}
}

func TestAdminClient(t *testing.T) {

	seedBroker := sarama.NewMockBroker(t, 1)
	defer seedBroker.Close()

	seedBroker.SetHandlerByMap(map[string]sarama.MockResponse{
		"MetadataRequest": sarama.NewMockMetadataResponse(t).
			SetController(seedBroker.BrokerID()).
			SetBroker(seedBroker.Addr(), seedBroker.BrokerID()),
	})

	// mock broker does not support TLS ...
	admin, err := MakeAdminClient("test-client", nil, []string{seedBroker.Addr()})
	if err != nil {
		t.Fatal(err)
	}

	require.NoError(t, err)
	require.NotNil(t, admin)
}
