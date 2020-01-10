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
	"reflect"
	"testing"

	"k8s.io/client-go/dynamic"
	"knative.dev/eventing/test/e2e/helpers"
	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/resources"

	lib2 "knative.dev/eventing-contrib/test/lib"
	contribresources "knative.dev/eventing-contrib/test/lib/resources"
)

// This test take for granted that the kafka cluster already exists together with the test-topic topic
const (
	kafkaBootstrapUrl         = "my-cluster-kafka-bootstrap.kafka.svc:9092"
	kafkaTestTopic            = "test-topic"
	testPace                  = "1:10"
	kafkaPerformanceImageName = "kafka_performance"
	testWarmup                = "0"
)

var expectedStruct = helpers.PerformanceImageResults{
	SentEvents: 10, AcceptedEvents: 10, ReceivedEvents: 10, PublishFailuresEvents: 0, DeliveryFailuresEvents: 0,
}

func TestKafkaSource(t *testing.T) {
	helpers.TestWithPerformanceImage(t, 2, func(t *testing.T, consumerHostname string, aggregatorHostname string, client *lib.Client) {
		t.Logf("Creating KafkaSource")
		lib2.CreateKafkaSourceOrFail(client, contribresources.KafkaSource(
			kafkaBootstrapUrl,
			kafkaTestTopic,
			resources.ServiceRef(resources.PerfConsumerService),
		))

		client.Dynamic = dynamic.NewForConfigOrDie(client.Config)

		t.Logf("Waiting for all resources ready")
		client.WaitForAllTestResourcesReadyOrFail()

		t.Logf("Starting receiver pod")
		client.CreatePodOrFail(resources.PerformanceImageReceiverPod(kafkaPerformanceImageName, testPace, testWarmup, aggregatorHostname))
		client.WaitForServiceEndpointsOrFail(resources.PerfConsumerService, 1)

		t.Logf("Starting sender pod")
		client.CreatePodOrFail(contribresources.KafkaPerformanceImageSenderPod(testPace, testWarmup, kafkaBootstrapUrl, kafkaTestTopic, aggregatorHostname))
	}, func(t *testing.T, results *helpers.PerformanceImageResults) {
		t.Logf("Received results %+v", *results)
		if !reflect.DeepEqual(*results, expectedStruct) {
			t.Fatalf("Have %+v, Expecting %+v", *results, expectedStruct)
		}
	})
}
