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
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"io/ioutil"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Shopify/sarama"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	sourcesv1alpha1 "github.com/knative/eventing-sources/contrib/kafka/pkg/apis/sources/v1alpha1"
	"github.com/knative/eventing-sources/pkg/kncloudevents"
)

func TestPostMessage_ServeHTTP(t *testing.T) {
	testCases := map[string]struct {
		sink              func(http.ResponseWriter, *http.Request)
		reqBody           string
		attributes        map[string]string
		expectedEventType string
		error             bool
	}{
		"accepted": {
			sink:    sinkAccepted,
			reqBody: `{"key":"value"}`,
		},
		"rejected": {
			sink:    sinkRejected,
			reqBody: `{"key":"value"}`,
			error:   true,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			h := &fakeHandler{
				handler: tc.sink,
			}
			sinkServer := httptest.NewServer(h)
			defer sinkServer.Close()

			a := &Adapter{
				Topics:           "topic1,topic2",
				BootstrapServers: "server1,server2",
				ConsumerGroup:    "group",
				SinkURI:          sinkServer.URL,
				client: func() client.Client {
					c, _ := kncloudevents.NewDefaultClient(sinkServer.URL)
					return c
				}(),
			}

			data, err := json.Marshal(map[string]string{"key": "value"})
			if err != nil {
				t.Errorf("unexpected error, %v", err)
			}

			m := &sarama.ConsumerMessage{
				Key:       []byte("key"),
				Topic:     "topic1",
				Value:     data,
				Partition: 1,
				Offset:    2,
				Timestamp: time.Now(),
			}

			err = a.postMessage(context.TODO(), m)

			if tc.error && err == nil {
				t.Errorf("expected error, but got %v", err)
			}

			et := h.header.Get("Ce-Type")

			expectedEventType := sourcesv1alpha1.KafkaEventType
			if tc.expectedEventType != "" {
				expectedEventType = tc.expectedEventType
			}

			if et != expectedEventType {
				t.Errorf("Expected eventtype '%q', but got '%q'", tc.expectedEventType, et)
			}
			if tc.reqBody != string(h.body) {
				t.Errorf("Expected request body '%q', but got '%q'", tc.reqBody, h.body)
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
	}{
		{
			name:    "all empty",
			wantNil: true,
		},
		{
			name:    "bad input",
			cert:    "x",
			key:     "y",
			caCert:  "z",
			wantErr: true,
		},
		{
			name:    "only cert",
			cert:    cert,
			wantNil: true,
		},
		{
			name:    "only key",
			key:     key,
			wantNil: true,
		},
		{
			name:       "cert and key",
			cert:       cert,
			key:        key,
			wantClient: true,
		},
		{
			name:       "only caCert",
			caCert:     cert,
			wantServer: true,
		},
		{
			name:       "cert, key, and caCert",
			cert:       cert,
			key:        key,
			caCert:     cert,
			wantClient: true,
			wantServer: true,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			c, err := newTLSConfig(tt.cert, tt.key, tt.caCert)
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

type fakeHandler struct {
	body   []byte
	header http.Header

	handler func(http.ResponseWriter, *http.Request)
}

func (h *fakeHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.header = r.Header
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "can not read body", http.StatusBadRequest)
		return
	}
	h.body = body

	defer r.Body.Close()
	h.handler(w, r)
}

func sinkAccepted(writer http.ResponseWriter, req *http.Request) {
	writer.WriteHeader(http.StatusOK)
}

func sinkRejected(writer http.ResponseWriter, _ *http.Request) {
	writer.WriteHeader(http.StatusRequestTimeout)
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
