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
	"testing"

	"github.com/cloudevents/sdk-go/v2/test"
	"github.com/google/uuid"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing-contrib/test/e2e/helpers"
	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/recordevents"
	"knative.dev/eventing/test/lib/resources"
	"knative.dev/pkg/tracker"

	contribtestlib "knative.dev/eventing-contrib/test/lib"
	contribresources "knative.dev/eventing-contrib/test/lib/resources"
)

func testKafkaBinding(t *testing.T, version string, messageKey string, messageHeaders map[string]string, messagePayload string, expectedData string) {
	client := testlib.Setup(t, true)

	kafkaTopicName := uuid.New().String()
	loggerPodName := "e2e-kafka-binding-event-logger"

	defer testlib.TearDown(client)

	helpers.MustCreateTopic(client, kafkaClusterName, kafkaClusterNamespace, kafkaTopicName)

	t.Logf("Creating EventRecord")
	eventTracker, _ := recordevents.StartEventRecordOrFail(context.Background(), client, loggerPodName)

	t.Logf("Creating KafkaSource %s", version)
	switch version {
	case "v1alpha1":
		contribtestlib.CreateKafkaSourceV1Alpha1OrFail(client, contribresources.KafkaSourceV1Alpha1(
			kafkaBootstrapUrl,
			kafkaTopicName,
			resources.ServiceRef(loggerPodName),
		))
	case "v1beta1":
		contribtestlib.CreateKafkaSourceV1Beta1OrFail(client, contribresources.KafkaSourceV1Beta1(
			kafkaBootstrapUrl,
			kafkaTopicName,
			resources.ServiceRef(loggerPodName),
		))
	default:
		t.Fatalf("Unknown KafkaSource version %s", version)
	}
	selector := map[string]string{
		"topic": kafkaTopicName,
	}

	t.Logf("Creating KafkaBinding %s", version)
	switch version {
	case "v1alpha1":
		contribtestlib.CreateKafkaBindingV1Alpha1OrFail(client, contribresources.KafkaBindingV1Alpha1(
			kafkaBootstrapUrl,
			&tracker.Reference{
				APIVersion: "batch/v1",
				Kind:       "Job",
				Selector: &metav1.LabelSelector{
					MatchLabels: selector,
				},
			},
		))
	case "v1beta1":
		contribtestlib.CreateKafkaBindingV1Beta1OrFail(client, contribresources.KafkaBindingV1Beta1(
			kafkaBootstrapUrl,
			&tracker.Reference{
				APIVersion: "batch/v1",
				Kind:       "Job",
				Selector: &metav1.LabelSelector{
					MatchLabels: selector,
				},
			},
		))
	default:
		t.Fatalf("Unknown KafkaSource version %s", version)
	}

	client.WaitForAllTestResourcesReadyOrFail(context.Background())

	helpers.MustPublishKafkaMessageViaBinding(client, selector, kafkaTopicName, messageKey, messageHeaders, messagePayload)

	eventTracker.AssertAtLeast(1, recordevents.MatchEvent(test.HasData([]byte(expectedData))))
}

func TestKafkaBinding(t *testing.T) {
	tests := map[string]struct {
		messageKey     string
		messageHeaders map[string]string
		messagePayload string
		expectedData   string
	}{
		"no_event": {
			messageKey: "0",
			messageHeaders: map[string]string{
				"content-type": "application/json",
			},
			messagePayload: `{"value":5}`,
			expectedData:   `{"value":5}`,
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
			expectedData: `{"hello":"Francesco"}`,
		},
	}
	for name, tc := range tests {
		tc := tc
		t.Run(name+"-v1alpha1", func(t *testing.T) {
			testKafkaBinding(t, "v1alpha1", tc.messageKey, tc.messageHeaders, tc.messagePayload, tc.expectedData)
		})
		t.Run(name+"-v1beta1", func(t *testing.T) {
			testKafkaBinding(t, "v1beta1", tc.messageKey, tc.messageHeaders, tc.messagePayload, tc.expectedData)
		})
	}
}
