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
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cloudevents/sdk-go/v2/types"
	"knative.dev/eventing/pkg/adapter"
	"knative.dev/pkg/source"

	"go.uber.org/zap"

	"github.com/Shopify/sarama"

	"knative.dev/eventing/pkg/kncloudevents"

	sourcesv1alpha1 "knative.dev/eventing-contrib/kafka/source/pkg/apis/sources/v1alpha1"
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
				"ce-id":      makeEventId(1, 2),
				"ce-time":    types.FormatTime(aTimestamp),
				"ce-type":    sourcesv1alpha1.KafkaEventType,
				"ce-source":  sourcesv1alpha1.KafkaEventSource("test", "test", "topic1"),
				"ce-subject": makeEventSubject(1, 2),
				"ce-key":     "key",
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
				"ce-id":      makeEventId(1, 2),
				"ce-time":    types.FormatTime(aTimestamp),
				"ce-type":    sourcesv1alpha1.KafkaEventType,
				"ce-source":  sourcesv1alpha1.KafkaEventSource("test", "test", "topic1"),
				"ce-subject": makeEventSubject(1, 2),
				"ce-key":     "-16771305",
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
				"ce-id":      makeEventId(1, 2),
				"ce-time":    types.FormatTime(aTimestamp),
				"ce-type":    sourcesv1alpha1.KafkaEventType,
				"ce-source":  sourcesv1alpha1.KafkaEventSource("test", "test", "topic1"),
				"ce-subject": makeEventSubject(1, 2),
				"ce-key":     "0.00000000000000000000000000000000000002536316309005082",
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
				"ce-id":      makeEventId(1, 2),
				"ce-time":    types.FormatTime(aTimestamp),
				"ce-type":    sourcesv1alpha1.KafkaEventType,
				"ce-source":  sourcesv1alpha1.KafkaEventSource("test", "test", "topic1"),
				"ce-subject": makeEventSubject(1, 2),
				"ce-key":     "AQoXFw==",
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
				"content-type": "application/cloudevents+json; charset=UTF-8",
			},
			expectedBody: string(mustJsonMarshal(t, map[string]interface{}{
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
			})),
			error: false,
		},
		"accepted_binary": {
			sink: sinkAccepted,
			message: &sarama.ConsumerMessage{
				Key:   []byte("key"),
				Topic: "topic1",
				Value: mustJsonMarshal(t, map[string]string{
					"hello": "Francesco",
				}),
				Partition: 0,
				Offset:    0,
				Headers: []*sarama.RecordHeader{{
					Key: []byte("content-type"), Value: []byte("application/json"),
				}, {
					Key: []byte("ce_specversion"), Value: []byte("1.0"),
				}, {
					Key: []byte("ce_type"), Value: []byte("com.github.pull.create"),
				}, {
					Key: []byte("ce_source"), Value: []byte("https://github.com/cloudevents/spec/pull"),
				}, {
					Key: []byte("ce_subject"), Value: []byte("123"),
				}, {
					Key: []byte("ce_id"), Value: []byte("A234-1234-1234"),
				}, {
					Key: []byte("ce_time"), Value: []byte("2018-04-05T17:31:00Z"),
				}, {
					Key: []byte("ce_comexampleextension1"), Value: []byte("value"),
				}, {
					Key: []byte("ce_comexampleothervalue"), Value: []byte("5"),
				}},
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
				"ce-id":      makeEventId(1, 2),
				"ce-time":    types.FormatTime(aTimestamp),
				"ce-type":    sourcesv1alpha1.KafkaEventType,
				"ce-source":  sourcesv1alpha1.KafkaEventSource("test", "test", "topic1"),
				"ce-subject": makeEventSubject(1, 2),
				"ce-key":     "key",
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

			s, err := kncloudevents.NewHttpMessageSender(nil, sinkServer.URL)
			if err != nil {
				t.Fatal(err)
			}

			a := &Adapter{
				config: &adapterConfig{
					EnvConfig: adapter.EnvConfig{
						SinkURI:   sinkServer.URL,
						Namespace: "test",
					},
					Topics:        []string{"topic1", "topic2"},
					ConsumerGroup: "group",
					Name:          "test",
				},
				httpMessageSender: s,
				logger:            zap.NewNop(),
				reporter:          statsReporter,
				keyTypeMapper:     getKeyTypeMapper(tc.keyTypeMapper),
			}

			_, err = a.Handle(context.TODO(), tc.message)

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
