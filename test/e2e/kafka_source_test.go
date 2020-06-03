//+build e2e

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

package e2e

import (
	"encoding/json"
	"sync"
	"testing"

	"github.com/google/uuid"
	"knative.dev/eventing-contrib/test/e2e/helpers"
	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/resources"

	lib2 "knative.dev/eventing-contrib/test/lib"
	contribresources "knative.dev/eventing-contrib/test/lib/resources"
)

// This test take for granted that the kafka cluster already exists together with the test-topic topic
const (
	kafkaBootstrapUrl     = "my-cluster-kafka-bootstrap.kafka.svc:9092"
	kafkaClusterName      = "my-cluster"
	kafkaClusterNamespace = "kafka"
)

func testKafkaSource(t *testing.T, messageKey string, messageHeaders map[string]string, messagePayload string, expectedCheckInLog string) {
	client := lib.Setup(t, true)

	kafkaTopicName := uuid.New().String()

	defer lib.TearDown(client)

	helpers.MustCreateTopic(client, kafkaClusterName, kafkaClusterNamespace, kafkaTopicName)

	loggers := []string{
		"e2e-kafka-source-event-logger-1",
		"e2e-kafka-source-event-logger-2",
		"e2e-kafka-source-event-logger-3",
	}

	var wg sync.WaitGroup
	wg.Add(len(loggers))
	for _, loggerPodName := range loggers {
		go func(loggerPodName string) {
			t.Logf("Creating EventLogger %s", loggerPodName)
			pod := resources.EventLoggerPod(loggerPodName)
			client.CreatePodOrFail(pod, lib.WithService(loggerPodName))

			t.Logf("Creating KafkaSource")
			lib2.CreateKafkaSourceOrFail(client, contribresources.KafkaSource(
				kafkaBootstrapUrl,
				kafkaTopicName,
				resources.ServiceRef(loggerPodName),
				contribresources.WithName(loggerPodName),
				contribresources.WithConsumerGroup(loggerPodName),
			))
			wg.Done()
		}(loggerPodName)
	}
	wg.Wait()

	client.WaitForAllTestResourcesReadyOrFail()

	n := 20
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func() {
			helpers.MustPublishKafkaMessage(client, kafkaBootstrapUrl, kafkaTopicName, messageKey, messageHeaders, messagePayload)
			wg.Done()
		}()
	}
	wg.Wait()

	// verify the logger service receives events
	for _, loggerPodName := range loggers {
		if err := client.CheckLog(loggerPodName, lib.CheckerContainsCount(expectedCheckInLog, n)); err != nil {
			t.Fatalf("String %q not found in logs of logger pod %q: %v", expectedCheckInLog, loggerPodName, err)
		}
	}
}

func TestKafkaSource(t *testing.T) {
	tests := map[string]struct {
		messageKey         string
		messageHeaders     map[string]string
		messagePayload     string
		expectedCheckInLog string
	}{
		"no_event": {
			messageKey: "0",
			messageHeaders: map[string]string{
				"content-type": "application/json",
			},
			messagePayload:     "{\"value\":5}",
			expectedCheckInLog: "\"value\": 5",
		},
		"simple_text_no_message_headers": {
			messageKey:         "0",
			messagePayload:     "simple 10",
			expectedCheckInLog: "simple 10",
		},
		"simple_text_no_message_headers_no_key": {
			messagePayload:     "simple 10",
			expectedCheckInLog: "simple 10",
		},
		"simple_text": {
			messageKey: "0",
			messageHeaders: map[string]string{
				"content-type": "text/plain",
			},
			messagePayload:     "simple 10",
			expectedCheckInLog: "simple 10",
		},
		"structured": {
			messageKey: "0",
			messageHeaders: map[string]string{
				"content-type": "application/cloudevents+json",
			},
			messagePayload: mustJsonMarshal(t, map[string]interface{}{
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
			expectedCheckInLog: "\"hello\": \"Francesco\"",
		},
	}
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			testKafkaSource(t, test.messageKey, test.messageHeaders, test.messagePayload, test.expectedCheckInLog)
		})
	}
}

func mustJsonMarshal(t *testing.T, val interface{}) string {
	data, err := json.Marshal(val)
	if err != nil {
		t.Errorf("unexpected error, %v", err)
	}
	return string(data)
}
