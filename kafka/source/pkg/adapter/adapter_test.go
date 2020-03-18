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

	cloudevents "github.com/cloudevents/sdk-go/legacy"
	"github.com/cloudevents/sdk-go/legacy/pkg/cloudevents/types"
	"knative.dev/eventing/pkg/adapter"
	"knative.dev/pkg/source"

	"go.uber.org/zap"

	"github.com/Shopify/sarama"
	"github.com/cloudevents/sdk-go/legacy/pkg/cloudevents/client"

	sourcesv1alpha1 "knative.dev/eventing-contrib/kafka/source/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing-contrib/pkg/kncloudevents"
)

func TestPostMessage_ServeHTTP(t *testing.T) {
	aTimestamp := time.Now()

	testCases := map[string]struct {
		sink            func(http.ResponseWriter, *http.Request)
		keyTypeMapper   string
		message         *sarama.ConsumerMessage
		expectedHeaders map[string]string
		expectedBody    string
		error           bool
	}{
		"accepted_simple": {
			sink: sinkAccepted,
			message: &sarama.ConsumerMessage{
				Key:       []byte("key"),
				Topic:     "topic1",
				Value:     mustJsonMarshal(t, map[string]string{"key": "value"}),
				Partition: 1,
				Offset:    2,
				Timestamp: aTimestamp,
			},
			expectedHeaders: map[string]string{
				"ce-id":        makeEventId(1, 2),
				"ce-time":      types.FormatTime(aTimestamp),
				"ce-type":      sourcesv1alpha1.KafkaEventType,
				"ce-source":    sourcesv1alpha1.KafkaEventSource("test", "test", "topic1"),
				"ce-subject":   makeEventSubject(1, 2),
				"ce-key":       "key",
				"content-type": cloudevents.ApplicationJSON,
			},
			expectedBody: `{"key":"value"}`,
			error:        false,
		},
		"accepted_int_key": {
			sink: sinkAccepted,
			message: &sarama.ConsumerMessage{
				Key:       []byte{255, 0, 23, 23},
				Topic:     "topic1",
				Value:     mustJsonMarshal(t, map[string]string{"key": "value"}),
				Partition: 1,
				Offset:    2,
				Timestamp: aTimestamp,
			},
			expectedHeaders: map[string]string{
				"ce-id":        makeEventId(1, 2),
				"ce-time":      types.FormatTime(aTimestamp),
				"ce-type":      sourcesv1alpha1.KafkaEventType,
				"ce-source":    sourcesv1alpha1.KafkaEventSource("test", "test", "topic1"),
				"ce-subject":   makeEventSubject(1, 2),
				"ce-key":       "-16771305",
				"content-type": cloudevents.ApplicationJSON,
			},
			expectedBody:  `{"key":"value"}`,
			error:         false,
			keyTypeMapper: "int",
		},
		"accepted_float_key": {
			sink: sinkAccepted,
			message: &sarama.ConsumerMessage{
				Key:       []byte{1, 10, 23, 23},
				Topic:     "topic1",
				Value:     mustJsonMarshal(t, map[string]string{"key": "value"}),
				Partition: 1,
				Offset:    2,
				Timestamp: aTimestamp,
			},
			expectedHeaders: map[string]string{
				"ce-id":        makeEventId(1, 2),
				"ce-time":      types.FormatTime(aTimestamp),
				"ce-type":      sourcesv1alpha1.KafkaEventType,
				"ce-source":    sourcesv1alpha1.KafkaEventSource("test", "test", "topic1"),
				"ce-subject":   makeEventSubject(1, 2),
				"ce-key":       "0.00000000000000000000000000000000000002536316309005082",
				"content-type": cloudevents.ApplicationJSON,
			},
			expectedBody:  `{"key":"value"}`,
			error:         false,
			keyTypeMapper: "float",
		},
		"accepted_byte-array_key": {
			sink: sinkAccepted,
			message: &sarama.ConsumerMessage{
				Key:       []byte{1, 10, 23, 23},
				Topic:     "topic1",
				Value:     mustJsonMarshal(t, map[string]string{"key": "value"}),
				Partition: 1,
				Offset:    2,
				Timestamp: aTimestamp,
			},
			expectedHeaders: map[string]string{
				"ce-id":        makeEventId(1, 2),
				"ce-time":      types.FormatTime(aTimestamp),
				"ce-type":      sourcesv1alpha1.KafkaEventType,
				"ce-source":    sourcesv1alpha1.KafkaEventSource("test", "test", "topic1"),
				"ce-subject":   makeEventSubject(1, 2),
				"ce-key":       "AQoXFw==",
				"content-type": cloudevents.ApplicationJSON,
			},
			expectedBody:  `{"key":"value"}`,
			error:         false,
			keyTypeMapper: "byte-array",
		},
		"accepted_complex": {
			sink: sinkAccepted,
			message: &sarama.ConsumerMessage{
				Key:   []byte("key"),
				Topic: "topic1",
				Headers: []*sarama.RecordHeader{
					{
						Key: []byte("hello"), Value: []byte("world"),
					},
					{
						Key: []byte("name"), Value: []byte("Francesco"),
					},
				},
				Value:     mustJsonMarshal(t, map[string]string{"key": "value"}),
				Partition: 1,
				Offset:    2,
				Timestamp: aTimestamp,
			},
			expectedHeaders: map[string]string{
				"ce-id":               makeEventId(1, 2),
				"ce-time":             types.FormatTime(aTimestamp),
				"ce-type":             sourcesv1alpha1.KafkaEventType,
				"ce-source":           sourcesv1alpha1.KafkaEventSource("test", "test", "topic1"),
				"ce-subject":          makeEventSubject(1, 2),
				"ce-key":              "key",
				"content-type":        cloudevents.ApplicationJSON,
				"ce-kafkaheaderhello": "world",
				"ce-kafkaheadername":  "Francesco",
			},
			expectedBody: `{"key":"value"}`,
			error:        false,
		},
		"accepted_fix_bad_headers": {
			sink: sinkAccepted,
			message: &sarama.ConsumerMessage{
				Key:   []byte("key"),
				Topic: "topic1",
				Headers: []*sarama.RecordHeader{
					{
						Key: []byte("hello-bla"), Value: []byte("world"),
					},
					{
						Key: []byte("name"), Value: []byte("Francesco"),
					},
				},
				Value:     mustJsonMarshal(t, map[string]string{"key": "value"}),
				Partition: 1,
				Offset:    2,
				Timestamp: aTimestamp,
			},
			expectedHeaders: map[string]string{
				"ce-id":                  makeEventId(1, 2),
				"ce-time":                types.FormatTime(aTimestamp),
				"ce-type":                sourcesv1alpha1.KafkaEventType,
				"ce-source":              sourcesv1alpha1.KafkaEventSource("test", "test", "topic1"),
				"ce-subject":             makeEventSubject(1, 2),
				"ce-key":                 "key",
				"content-type":           cloudevents.ApplicationJSON,
				"ce-kafkaheaderhellobla": "world",
				"ce-kafkaheadername":     "Francesco",
			},
			expectedBody: `{"key":"value"}`,
			error:        false,
		},
		"accepted_structured": {
			sink: sinkAccepted,
			message: &sarama.ConsumerMessage{
				Key:   []byte("key"),
				Topic: "topic1",
				Value: mustJsonMarshal(t, map[string]interface{}{
					"specversion":          "1.0",
					"type":                 "com.github.pull.create",
					"source":               "https://github.com/cloudevents/spec/pull",
					"subject":              "123",
					"id":                   "A234-1234-1234",
					"time":                 "2018-04-05T17:31:00Z",
					"comexampleextension1": "value",
					"comexampleothervalue": 5,
					"datacontenttype":      "application/json",
					"data": map[string]string{
						"hello": "Francesco",
					},
				}),
				Partition: 0,
				Offset:    0,
				Headers: []*sarama.RecordHeader{
					{
						Key: []byte("content-type"), Value: []byte("application/cloudevents+json; charset=UTF-8"),
					},
				},
				Timestamp: aTimestamp,
			},
			expectedHeaders: map[string]string{
				"ce-specversion":          "1.0",
				"ce-id":                   "A234-1234-1234",
				"ce-time":                 "2018-04-05T17:31:00Z",
				"ce-type":                 "com.github.pull.create",
				"ce-subject":              "123",
				"ce-source":               "https://github.com/cloudevents/spec/pull",
				"ce-comexampleextension1": "value",
				"ce-comexampleothervalue": "5",
				"content-type":            "application/json",
			},
			expectedBody: `{"hello":"Francesco"}`,
			error:        false,
		},
		"rejected": {
			sink: sinkRejected,
			message: &sarama.ConsumerMessage{
				Key:       []byte("key"),
				Topic:     "topic1",
				Value:     mustJsonMarshal(t, map[string]string{"key": "value"}),
				Partition: 1,
				Offset:    2,
				Timestamp: aTimestamp,
			},
			expectedHeaders: map[string]string{
				"ce-id":        makeEventId(1, 2),
				"ce-time":      types.FormatTime(aTimestamp),
				"ce-type":      sourcesv1alpha1.KafkaEventType,
				"ce-source":    sourcesv1alpha1.KafkaEventSource("test", "test", "topic1"),
				"ce-subject":   makeEventSubject(1, 2),
				"ce-key":       "key",
				"content-type": cloudevents.ApplicationJSON,
			},
			expectedBody: `{"key":"value"}`,
			error:        true,
		},
	}

	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			h := &fakeHandler{
				handler: tc.sink,
			}
			sinkServer := httptest.NewServer(h)
			defer sinkServer.Close()

			statsReporter, _ := source.NewStatsReporter()

			a := &Adapter{
				config: &adapterConfig{
					EnvConfig: adapter.EnvConfig{
						SinkURI:   sinkServer.URL,
						Namespace: "test",
					},
					Topics:           "topic1,topic2",
					BootstrapServers: "server1,server2",
					ConsumerGroup:    "group",
					Name:             "test",
					Net:              AdapterNet{},
				},
				ceClient: func() client.Client {
					c, _ := kncloudevents.NewDefaultClient(sinkServer.URL)
					return c
				}(),
				logger:        zap.NewNop(),
				reporter:      statsReporter,
				keyTypeMapper: getKeyTypeMapper(tc.keyTypeMapper),
			}

			_, err := a.Handle(context.TODO(), tc.message)

			if tc.error && err == nil {
				t.Errorf("expected error, but got %v", err)
			}

			// Check headers
			for k, expected := range tc.expectedHeaders {
				actual := h.header.Get(k)
				if actual != expected {
					t.Errorf("Expected header with key %s: '%q', but got '%q'", k, expected, actual)
				}
			}

			// Check body
			if tc.expectedBody != string(h.body) {
				t.Errorf("Expected request body '%q', but got '%q'", tc.expectedBody, h.body)
			}
		})
	}
}

func mustJsonMarshal(t *testing.T, val interface{}) []byte {
	data, err := json.Marshal(val)
	if err != nil {
		t.Errorf("unexpected error, %v", err)
	}
	return data
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

func TestAdapterStartFailure(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()

	adapter := &Adapter{
		config: &adapterConfig{
			EnvConfig: adapter.EnvConfig{
				SinkURI:   "example.com",
				Namespace: "test",
			},
			Topics:           "topic1,topic2",
			BootstrapServers: "server1,server2",
			ConsumerGroup:    "group",
			Name:             "test",
			Net:              AdapterNet{},
		},
	}

	_ = adapter.Start(make(chan struct{}))
}
