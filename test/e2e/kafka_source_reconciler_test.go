//+build e2e

/*
Copyright 2020 The Knative Authors

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

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"

	testlib "knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/resources"
	"knative.dev/pkg/apis"
	pkgTest "knative.dev/pkg/test"

	sourcesv1beta1 "knative.dev/eventing-contrib/kafka/source/pkg/apis/sources/v1beta1"
	"knative.dev/eventing-contrib/test/e2e/helpers"
	contribtestlib "knative.dev/eventing-contrib/test/lib"
	contribresources "knative.dev/eventing-contrib/test/lib/resources"
)

const (
	rtKafkaSourceName    = "e2e-rt-kafka-source"
	rtChannelName        = "e2e-rt-channel"
	rtKafkaConsumerGroup = "e2e-rt-cg"
	rtKafkaTopicName     = "e2e-rt-topic"
)

//TestKafkaSourceReconciler tests various kafka source reconciler statuses
//RT is short for reconciler test
func TestKafkaSourceReconciler(t *testing.T) {
	client := testlib.Setup(t, true)
	defer testlib.TearDown(client)

	for _, test := range []struct {
		name             string
		action           func(c *testlib.Client)
		expectedStatuses sets.String
		wantRADepCount   int
	}{{
		"create_kafka_source",
		createKafkaSourceWithSinkMissing,
		sets.NewString("NotFound"),
		0,
	}, {
		"create_sink",
		createChannel,
		sets.NewString(""),
		1,
	}, {
		"delete_sink",
		deleteChannel,
		sets.NewString("NotFound"),
		0,
	}, {
		"create_sink_after_delete",
		createChannel,
		sets.NewString(""),
		1,
	}} {
		t.Run(test.name, func(t *testing.T) {
			testKafkaSourceReconciler(client, test.name, test.action, test.expectedStatuses, test.wantRADepCount)
		})
	}
}

func testKafkaSourceReconciler(c *testlib.Client, name string, doAction func(c *testlib.Client), expectedStatuses sets.String, wantRADepCount int) {
	doAction(c)

	if err := helpers.CheckKafkaSourceState(context.Background(), c, rtKafkaSourceName, func(ks *sourcesv1beta1.KafkaSource) (bool, error) {
		ready := ks.Status.GetCondition(apis.ConditionReady)
		if ready != nil {
			if expectedStatuses.Has(ready.Reason) {
				return true, nil
			}
		}
		return false, nil
	}); err != nil {
		c.T.Fatalf("Failed to validate kafkasource state, expected status : %v, err : %v", expectedStatuses.UnsortedList(), err)
	}

	if err := helpers.CheckRADeployment(context.Background(), c, rtKafkaSourceName, func(deps *appsv1.DeploymentList) (bool, error) {
		if len(deps.Items) == wantRADepCount {
			return true, nil
		}
		return false, nil
	}); err != nil {
		c.T.Fatal("Failed to validate adapter deployment state:", err)
	}
}

func createKafkaSourceWithSinkMissing(c *testlib.Client) {
	helpers.MustCreateTopic(c, kafkaClusterName, kafkaClusterNamespace, rtKafkaTopicName)

	contribtestlib.CreateKafkaSourceV1Beta1OrFail(c, contribresources.KafkaSourceV1Beta1(
		kafkaBootstrapUrl,
		rtKafkaTopicName,
		pkgTest.CoreV1ObjectReference(resources.InMemoryChannelKind, resources.MessagingAPIVersion, rtChannelName),
		contribresources.WithNameV1Beta1(rtKafkaSourceName),
		contribresources.WithConsumerGroupV1Beta1(rtKafkaConsumerGroup),
	))
}

func createChannel(c *testlib.Client) {
	c.CreateChannelOrFail(rtChannelName, &metav1.TypeMeta{
		APIVersion: resources.MessagingAPIVersion,
		Kind:       resources.InMemoryChannelKind,
	})
	c.WaitForAllTestResourcesReadyOrFail(context.Background())
}

func deleteChannel(c *testlib.Client) {
	contribtestlib.DeleteResourceOrFail(context.Background(), c, rtChannelName, helpers.ImcGVR)
}
