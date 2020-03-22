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
	"encoding/json"
	"io/ioutil"
	"net/http"
	"strconv"
	"testing"
	"time"

	"knative.dev/eventing/pkg/adapter"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/pkg/source"

	"github.com/Shopify/sarama"
	"go.uber.org/zap"
)

// Run with go test -v ./kafka/source/pkg/adapter/ -gcflags="-N -l" -test.benchtime 2s -benchmem -run=Handle -bench=.

type RoundTripFunc func(req *http.Request) *http.Response

func (f RoundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req), nil
}

func NewTestClient(fn RoundTripFunc) *http.Client {
	return &http.Client{
		Transport: RoundTripFunc(fn),
	}
}

func BenchmarkHandle(b *testing.B) {
	sinkUrl := "http://localhost:8080" // This address doesn't matter, it's not used b/c we mock the client transport

	data, err := json.Marshal(map[string]string{"key": "value"})
	if err != nil {
		b.Errorf("unexpected error, %v", err)
	}

	s := kncloudevents.HttpMessageSender{
		Client: NewTestClient(func(req *http.Request) *http.Response {
			return &http.Response{
				StatusCode: 202,
				Header:     make(http.Header),
				Body:       ioutil.NopCloser(bytes.NewReader([]byte{})),
			}
		}),
		Target: sinkUrl,
	}
	if err != nil {
		b.Fatal(err)
	}

	statsReporter, _ := source.NewStatsReporter()

	a := &Adapter{
		config: &adapterConfig{
			EnvConfig: adapter.EnvConfig{
				SinkURI:   sinkUrl,
				Namespace: "test",
			},
			Topics:           "topic1,topic2",
			BootstrapServers: "server1,server2",
			ConsumerGroup:    "group",
			Name:             "test",
			Net:              AdapterNet{},
		},
		httpMessageSender: &s,
		logger:            zap.NewNop(),
		keyTypeMapper:     getKeyTypeMapper(""),
		reporter:          statsReporter,
	}
	b.SetParallelism(1)
	b.Run("Baseline", func(b *testing.B) {
		baseline(b, a, data)
	})
	b.Run("Benchmark Handle", func(b *testing.B) {
		benchmarkHandle(b, a, data)
	})

}

// Avoid DCE
var Message *sarama.ConsumerMessage
var ABool bool
var AError error

// Baseline is required to understand how much time/memory is needed to allocate the sarama.ConsumerMessage
func baseline(b *testing.B, adapter *Adapter, payload []byte) {
	for i := 0; i < b.N; i++ {
		Message = &sarama.ConsumerMessage{
			Key:       []byte(strconv.Itoa(i)),
			Topic:     "topic1",
			Value:     payload,
			Partition: 1,
			Offset:    int64(i + 1),
			Timestamp: time.Now(),
		}
	}
}

func benchmarkHandle(b *testing.B, adapter *Adapter, payload []byte) {
	for i := 0; i < b.N; i++ {
		Message := &sarama.ConsumerMessage{
			Key:       []byte(strconv.Itoa(i)),
			Topic:     "topic1",
			Value:     payload,
			Partition: 1,
			Offset:    int64(i + 1),
			Timestamp: time.Now(),
		}
		ABool, AError = adapter.Handle(context.TODO(), Message)
		if AError != nil {
			panic(AError)
		}
	}
}
