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
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"knative.dev/eventing/pkg/adapter"

	"github.com/Shopify/sarama"
	"github.com/cloudevents/sdk-go/legacy/pkg/cloudevents/client"
	"go.uber.org/zap"

	"knative.dev/eventing-contrib/pkg/kncloudevents"
)

// Run with go test -v ./kafka/source/pkg/adapter/ -gcflags="-N -l" -test.benchtime 2s -benchmem -run=Handle -bench=.

type benchHandler struct{}

func (b benchHandler) ServeHTTP(writer http.ResponseWriter, req *http.Request) {
	writer.WriteHeader(http.StatusOK)
}

func BenchmarkHandle(b *testing.B) {
	// Start receiving server
	sinkServer := httptest.NewServer(benchHandler{})
	defer sinkServer.Close()

	data, err := json.Marshal(map[string]string{"key": "value"})
	if err != nil {
		b.Errorf("unexpected error, %v", err)
	}

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
		keyTypeMapper: getKeyTypeMapper(""),
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
