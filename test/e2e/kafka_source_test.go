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
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	cloudevents "github.com/cloudevents/sdk-go/v2"
	. "github.com/cloudevents/sdk-go/v2/test"
	cetypes "github.com/cloudevents/sdk-go/v2/types"
	"github.com/google/uuid"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"

	sourcesv1alpha1 "knative.dev/eventing-contrib/kafka/source/pkg/apis/sources/v1alpha1"
	sourcesv1beta1 "knative.dev/eventing-contrib/kafka/source/pkg/apis/sources/v1beta1"
	"knative.dev/eventing-contrib/test/e2e/helpers"
	contribtestlib "knative.dev/eventing-contrib/test/lib"
	contribresources "knative.dev/eventing-contrib/test/lib/resources"
)

const (
	kafkaBootstrapUrl     = "my-cluster-kafka-bootstrap.kafka.svc:9092"
	kafkaClusterName      = "my-cluster"
	kafkaClusterNamespace = "kafka"
)

func testKafkaSource(t *testing.T, name string, version string, messageKey string, messageHeaders map[string]string, messagePayload string, matcherGen func(cloudEventsSourceName, cloudEventsEventType string) EventMatcher) {
	name = fmt.Sprintf("%s-%s", name, version)

	var (
		kafkaTopicName     = uuid.New().String()
		consumerGroup      = uuid.New().String()
		recordEventPodName = "e2e-kafka-recordevent-" + strings.ReplaceAll(name, "_", "-")
		kafkaSourceName    = "e2e-kafka-source-" + strings.ReplaceAll(name, "_", "-")
	)

	client := testlib.Setup(t, true)
	defer testlib.TearDown(client)

	helpers.MustCreateTopic(client, kafkaClusterName, kafkaClusterNamespace, kafkaTopicName)

	eventTracker, _ := recordevents.StartEventRecordOrFail(context.Background(), client, recordEventPodName)

	var (
		cloudEventsSourceName string
		cloudEventsEventType  string
	)

	t.Logf("Creating KafkaSource %s", version)
	switch version {
	case "v1alpha1":
		contribtestlib.CreateKafkaSourceV1Alpha1OrFail(client, contribresources.KafkaSourceV1Alpha1(
			kafkaBootstrapUrl,
			kafkaTopicName,
			resources.ServiceRef(recordEventPodName),
			contribresources.WithNameV1Alpha1(kafkaSourceName),
			contribresources.WithConsumerGroupV1Alpha1(consumerGroup),
		))
		cloudEventsSourceName = sourcesv1alpha1.KafkaEventSource(client.Namespace, kafkaSourceName, kafkaTopicName)
		cloudEventsEventType = sourcesv1alpha1.KafkaEventType
	case "v1beta1":
		contribtestlib.CreateKafkaSourceV1Beta1OrFail(client, contribresources.KafkaSourceV1Beta1(
			kafkaBootstrapUrl,
			kafkaTopicName,
			resources.ServiceRef(recordEventPodName),
			contribresources.WithNameV1Beta1(kafkaSourceName),
			contribresources.WithConsumerGroupV1Beta1(consumerGroup),
		))
		cloudEventsSourceName = sourcesv1beta1.KafkaEventSource(client.Namespace, kafkaSourceName, kafkaTopicName)
		cloudEventsEventType = sourcesv1beta1.KafkaEventType
	default:
		t.Fatalf("Unknown KafkaSource version %s", version)
	}

	client.WaitForAllTestResourcesReadyOrFail(context.Background())

	helpers.MustPublishKafkaMessage(client, kafkaBootstrapUrl, kafkaTopicName, messageKey, messageHeaders, messagePayload)

	eventTracker.AssertExact(1, recordevents.MatchEvent(matcherGen(cloudEventsSourceName, cloudEventsEventType)))
}

func TestKafkaSource(t *testing.T) {
	time, _ := cetypes.ParseTime("2018-04-05T17:31:00Z")

	tests := map[string]struct {
		messageKey     string
		messageHeaders map[string]string
		messagePayload string
		matcherGen     func(cloudEventsSourceName, cloudEventsEventType string) EventMatcher
	}{
		"no_event": {
			messageKey: "0",
			messageHeaders: map[string]string{
				"content-type": "application/json",
			},
			messagePayload: `{"value":5}`,
			matcherGen: func(cloudEventsSourceName, cloudEventsEventType string) EventMatcher {
				return AllOf(
					HasSource(cloudEventsSourceName),
					HasType(cloudEventsEventType),
					HasDataContentType("application/json"),
					HasData([]byte(`{"value":5}`)),
					HasExtension("key", "0"),
				)
			},
		},
		"no_event_no_content_type": {
			messageKey:     "0",
			messagePayload: `{"value":5}`,
			matcherGen: func(cloudEventsSourceName, cloudEventsEventType string) EventMatcher {
				return AllOf(
					HasSource(cloudEventsSourceName),
					HasType(cloudEventsEventType),
					HasData([]byte(`{"value":5}`)),
					HasExtension("key", "0"),
				)
			},
		},
		"no_event_no_content_type_no_key": {
			messagePayload: `{"value":5}`,
			matcherGen: func(cloudEventsSourceName, cloudEventsEventType string) EventMatcher {
				return AllOf(
					HasSource(cloudEventsSourceName),
					HasType(cloudEventsEventType),
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
			matcherGen: func(cloudEventsSourceName, cloudEventsEventType string) EventMatcher {
				return AllOf(
					HasSource(cloudEventsSourceName),
					HasType(cloudEventsEventType),
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
			matcherGen: func(cloudEventsSourceName, cloudEventsEventType string) EventMatcher {
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
			matcherGen: func(cloudEventsSourceName, cloudEventsEventType string) EventMatcher {
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
		t.Run(name+"-v1alpha1", func(t *testing.T) {
			testKafkaSource(t, name, "v1alpha1", test.messageKey, test.messageHeaders, test.messagePayload, test.matcherGen)
		})
		t.Run(name+"-v1beta1", func(t *testing.T) {
			testKafkaSource(t, name, "v1beta1", test.messageKey, test.messageHeaders, test.messagePayload, test.matcherGen)
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
