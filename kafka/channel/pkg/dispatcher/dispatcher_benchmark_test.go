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

package dispatcher

import (
	"context"
	"math/rand"
	"testing"
	"time"

	"github.com/Shopify/sarama"
	cloudevents "github.com/cloudevents/sdk-go/legacy"
	eventingchannels "knative.dev/eventing/pkg/channel"

	contribchannels "knative.dev/eventing-contrib/pkg/channel"
)

func mockTopicFunc(separator, namespace, name string) string {
	return ""
}

func BenchmarkToKafkaMessage(b *testing.B) {
	ref := eventingchannels.ChannelReference{Namespace: "namespace-test", Name: "name-test"}
	b.SetParallelism(1)
	b.ReportAllocs()
	b.Run("Baseline toKafkaMessage", func(b *testing.B) {
		baselineToKafkaMessage(b)
	})
	b.Run("Benchmark toKafkaMessage", func(b *testing.B) {
		benchmarkToKafkaMessage(b, ref)
	})
	b.Run("Baseline fromKafkaMessage", func(b *testing.B) {
		baselineFromKafkaMessage(b)
	})
	b.Run("Benchmark fromKafkaMessage", func(b *testing.B) {
		benchmarkFromKafkaMessage(b)
	})

}

// Avoid DCE
var message contribchannels.Message
var event cloudevents.Event
var saramaProducerMessage *sarama.ProducerMessage
var saramaConsumerMessage *sarama.ConsumerMessage

var seededRand *rand.Rand = rand.New(rand.NewSource(time.Now().UnixNano()))

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func randString(length uint) string {
	return string(randBytes(length))
}

func randBytes(length uint) []byte {
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return b
}

func genChannelMessage(payloadSize uint, headersNumber uint, headersSize uint) contribchannels.Message {
	payload := randBytes(payloadSize)

	headers := make(map[string]string, headersNumber)
	for i := uint(0); i < headersNumber; i++ {
		headers[randString(10)] = randString(headersSize)
	}

	return contribchannels.Message{
		Payload: payload,
		Headers: headers,
	}
}

func genChannelEvent(payloadSize uint, headersNumber uint, headersSize uint) cloudevents.Event {

	payload := randBytes(payloadSize)
	event := cloudevents.NewEvent(cloudevents.VersionV1)
	event.SetData(payload)
	event.SetID(string(randBytes(10)))
	event.SetSource("genChannelEvent")
	event.SetType("genChannelEventType")
	for i := uint(0); i < headersNumber; i++ {
		event.SetExtension(string(randBytes(10)), randBytes(headersSize))
	}
	return event
}

func genKafkaMessage(payloadSize uint, headersNumber uint, headersSize uint) *sarama.ConsumerMessage {
	payload := randBytes(payloadSize)

	headers := make([]*sarama.RecordHeader, headersNumber)
	for i := uint(0); i < headersNumber; i++ {
		headers[i] = &sarama.RecordHeader{Key: randBytes(10), Value: randBytes(headersSize)}
	}

	return &sarama.ConsumerMessage{
		Value:   payload,
		Headers: headers,
	}
}

func baselineToKafkaMessage(b *testing.B) {
	for i := 0; i < b.N; i++ {
		event = genChannelEvent(1024, 10, 64)
	}
}

func benchmarkToKafkaMessage(b *testing.B, ref eventingchannels.ChannelReference) {
	for i := 0; i < b.N; i++ {
		event := genChannelEvent(1024, 10, 64)
		saramaProducerMessage = toKafkaMessage(context.TODO(), ref, event, mockTopicFunc)
	}
}

func baselineFromKafkaMessage(b *testing.B) {
	for i := 0; i < b.N; i++ {
		saramaConsumerMessage = genKafkaMessage(1024, 10, 64)
	}
}

func benchmarkFromKafkaMessage(b *testing.B) {
	for i := 0; i < b.N; i++ {
		saramaConsumerMessage = genKafkaMessage(1024, 10, 64)
		event = *fromKafkaMessage(context.TODO(), saramaConsumerMessage)
	}
}
