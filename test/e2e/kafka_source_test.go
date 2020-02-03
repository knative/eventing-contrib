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
	"testing"

	"knative.dev/eventing-contrib/test/e2e/helpers"
	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/resources"

	lib2 "knative.dev/eventing-contrib/test/lib"
	contribresources "knative.dev/eventing-contrib/test/lib/resources"
)

// This test take for granted that the kafka cluster already exists together with the test-topic topic
const (
	kafkaBootstrapUrl = "my-cluster-kafka-bootstrap.kafka.svc:9092"
	kafkaTestTopic    = "test-topic"
)

func TestKafkaSource(t *testing.T) {
	client := lib.Setup(t, true)
	defer lib.TearDown(client)

	loggerPodName := "e2e-kafka-source-event-logger"

	t.Logf("Creating EventLogger")
	pod := resources.EventLoggerPod(loggerPodName)
	client.CreatePodOrFail(pod, lib.WithService(loggerPodName))

	t.Logf("Creating KafkaSource")
	lib2.CreateKafkaSourceOrFail(client, contribresources.KafkaSource(
		kafkaBootstrapUrl,
		kafkaTestTopic,
		resources.ServiceRef(loggerPodName),
	))

	client.WaitForAllTestResourcesReadyOrFail()

	eventPayload := "{\"value\":5}"

	helpers.MustPublishKafkaMessage(client, kafkaBootstrapUrl, kafkaTestTopic, "0", map[string]string{}, eventPayload)

	// verify the logger service receives the event
	if err := client.CheckLog(loggerPodName, lib.CheckerContains(eventPayload)); err != nil {
		t.Fatalf("String %q not found in logs of logger pod %q: %v", eventPayload, loggerPodName, err)
	}

}
