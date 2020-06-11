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
	"strings"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	. "github.com/cloudevents/sdk-go/v2/test"
	cetypes "github.com/cloudevents/sdk-go/v2/types"
	"github.com/google/uuid"
	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"

	sourcesv1alpha1 "knative.dev/eventing-contrib/kafka/source/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing-contrib/test/e2e/helpers"
	contriblib "knative.dev/eventing-contrib/test/lib"
	contribresources "knative.dev/eventing-contrib/test/lib/resources"
)

// This test take for granted that the kafka cluster already exists together with the test-topic topic
const (
	kafkaBootstrapUrl     = "my-cluster-kafka-bootstrap.kafka.svc:9092"
	kafkaClusterName      = "my-cluster"
	kafkaClusterNamespace = "kafka"
)

func testKafkaSource(t *testing.T, name string, messageKey string, messageHeaders map[string]string, messagePayload string, matcherGen func(namespace, kafkaSourceName, topic string) EventMatcher) {
	var (
		kafkaTopicName     = uuid.New().String()
		consumerGroup      = uuid.New().String()
		recordEventPodName = "e2e-kafka-recordevent-" + strings.ReplaceAll(name, "_", "-")
		kafkaSourceName    = "e2e-kafka-source-" + strings.ReplaceAll(name, "_", "-")
	)

	client := lib.Setup(t, true)
	defer lib.TearDown(client)

	helpers.MustCreateTopic(client, kafkaClusterName, kafkaClusterNamespace, kafkaTopicName)

	eventTracker, _ := recordevents.StartEventRecordOrFail(client, recordEventPodName)

	t.Logf("Creating KafkaSource")
	contriblib.CreateKafkaSourceOrFail(client, contribresources.KafkaSource(
		kafkaBootstrapUrl,
		kafkaTopicName,
		resources.ServiceRef(recordEventPodName),
		contribresources.WithName(kafkaSourceName),
		contribresources.WithConsumerGroup(consumerGroup),
	))

	client.WaitForAllTestResourcesReadyOrFail()

	helpers.MustPublishKafkaMessage(client, kafkaBootstrapUrl, kafkaTopicName, messageKey, messageHeaders, messagePayload)

	eventTracker.AssertExact(1, recordevents.MatchEvent(matcherGen(client.Namespace, kafkaSourceName, kafkaTopicName)))
}

func TestKafkaSource(t *testing.T) {
	time, _ := cetypes.ParseTime("2018-04-05T17:31:00Z")

	tests := map[string]struct {
		messageKey     string
		messageHeaders map[string]string
		messagePayload string
		matcherGen     func(namespace, kafkaSourceName, topic string) EventMatcher
	}{
		"no_event_with_content_type": {
			messageKey: "0",
			messageHeaders: map[string]string{
				"content-type": "application/json",
			},
			messagePayload: `{"value":5}`,
			matcherGen: func(namespace, kafkaSourceName, topic string) EventMatcher {
				return AllOf(
					HasSource(sourcesv1alpha1.KafkaEventSource(namespace, kafkaSourceName, topic)),
					HasType(sourcesv1alpha1.KafkaEventType),
					HasDataContentType("application/json"),
					HasData([]byte(`{"value":5}`)),
					HasExtension("key", "0"),
				)
			},
		},
		"no_event_without_content_type": {
			messageKey:     "0",
			messagePayload: `{"value":5}`,
			matcherGen: func(namespace, kafkaSourceName, topic string) EventMatcher {
				return AllOf(
					HasSource(sourcesv1alpha1.KafkaEventSource(namespace, kafkaSourceName, topic)),
					HasType(sourcesv1alpha1.KafkaEventType),
					HasData([]byte(`{"value":5}`)),
					HasExtension("key", "0"),
				)
			},
		},
		"no_event_without_content_type_and_without_key": {
			messagePayload: `{"value":5}`,
			matcherGen: func(namespace, kafkaSourceName, topic string) EventMatcher {
				return AllOf(
					HasSource(sourcesv1alpha1.KafkaEventSource(namespace, kafkaSourceName, topic)),
					HasType(sourcesv1alpha1.KafkaEventType),
					HasData([]byte(`{"value":5}`)),
				)
			},
		},
		"no_event_with_text_plain_body": {
			messageKey: "0",
			messageHeaders: map[string]string{
				"content-type": "text/plain",
			},
			messagePayload: "simple 10",
			matcherGen: func(namespace, kafkaSourceName, topic string) EventMatcher {
				return AllOf(
					HasSource(sourcesv1alpha1.KafkaEventSource(namespace, kafkaSourceName, topic)),
					HasType(sourcesv1alpha1.KafkaEventType),
					HasDataContentType("text/plain"),
					HasData([]byte("simple 10")),
					HasExtension("key", "0"),
				)
			},
		},
		"structured": {
			messageHeaders: map[string]string{
				"content-type": "application/cloudevents+json",
			},
			messagePayload: mustJsonMarshal(t, map[string]interface{}{
				"specversion":     "1.0",
				"type":            "com.github.pull.create",
				"source":          "https://github.com/cloudevents/spec/pull",
				"subject":         "123",
				"id":              "A234-1234-1234",
				"time":            "2018-04-05T17:31:00Z",
				"datacontenttype": "application/json",
				"data": map[string]string{
					"hello": "Francesco",
				},
				"comexampleextension1": "value",
				"comexampleothervalue": 5,
			}),
			matcherGen: func(namespace, kafkaSourceName, topic string) EventMatcher {
				return AllOf(
					HasSpecVersion(cloudevents.VersionV1),
					HasType("com.github.pull.create"),
					HasSource("https://github.com/cloudevents/spec/pull"),
					HasSubject("123"),
					HasId("A234-1234-1234"),
					HasTime(time),
					HasDataContentType("application/json"),
					HasData([]byte(`{"hello":"Francesco"}`)),
					HasExtension("comexampleextension1", "value"),
					HasExtension("comexampleothervalue", "5"),
				)
			},
		},
		"binary": {
			messageHeaders: map[string]string{
				"ce_specversion":          "1.0",
				"ce_type":                 "com.github.pull.create",
				"ce_source":               "https://github.com/cloudevents/spec/pull",
				"ce_subject":              "123",
				"ce_id":                   "A234-1234-1234",
				"ce_time":                 "2018-04-05T17:31:00Z",
				"content-type":            "application/json",
				"ce_comexampleextension1": "value",
				"ce_comexampleothervalue": "5",
			},
			messagePayload: mustJsonMarshal(t, map[string]string{
				"hello": "Francesco",
			}),
			matcherGen: func(namespace, kafkaSourceName, topic string) EventMatcher {
				return AllOf(
					HasSpecVersion(cloudevents.VersionV1),
					HasType("com.github.pull.create"),
					HasSource("https://github.com/cloudevents/spec/pull"),
					HasSubject("123"),
					HasId("A234-1234-1234"),
					HasTime(time),
					HasDataContentType("application/json"),
					HasData([]byte(`{"hello":"Francesco"}`)),
					HasExtension("comexampleextension1", "value"),
					HasExtension("comexampleothervalue", "5"),
				)
			},
		},
	}
	for name, test := range tests {
		test := test
		t.Run(name, func(t *testing.T) {
			testKafkaSource(t, name, test.messageKey, test.messageHeaders, test.messagePayload, test.matcherGen)
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
