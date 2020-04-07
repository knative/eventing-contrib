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
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/uuid"

	pkgTest "knative.dev/pkg/test"

	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/cloudevents"
	"knative.dev/eventing/test/lib/resources"
)

func TestBrokerRedeliveryFibonacci(t *testing.T) {
	BrokerRedeliveryWithTestRunner(t, channelTestRunner, "droplogeventsfibonacci")
}

func TestBrokerRedeliveryFirst(t *testing.T) {
	BrokerRedeliveryWithTestRunner(t, channelTestRunner, "droplogeventsfirst")
}

func BrokerRedeliveryWithTestRunner(
	t *testing.T,
	runner lib.ChannelTestRunner,
	dropLoggerImageName string,
	options ...lib.SetupClientOption,
) {

	const (
		senderName  = "e2e-channelredelivery-sender-"
		brokerName  = "e2e-channelredelivery-broker"
		triggerName = "e2e-channelredelivery-trigger"

		source1 = "test.knative.dev"
		type1   = "commit"
	)
	dropLoggerPodName := "e2e-channelredelivery-" + dropLoggerImageName

	events, ids := events(10, source1, type1)

	runner.RunTests(t, lib.FeatureRedelivery, func(t *testing.T, channel v1.TypeMeta) {

		client := lib.Setup(t, true, options...)
		defer lib.TearDown(client)

		t.Log("Create RBAC resources for brokers")
		client.CreateRBACResourcesForBrokers()

		t.Log("Create Pod", dropLoggerPodName)
		client.CreatePodOrFail(
			pod(dropLoggerImageName, dropLoggerPodName),
			lib.WithService(dropLoggerPodName),
		)

		t.Log("Create Trigger", triggerName)
		client.CreateTriggerOrFail(
			triggerName,
			resources.WithBroker(brokerName),
			resources.WithSubscriberServiceRefForTrigger(dropLoggerPodName),
		)

		t.Log("Create Broker", brokerName)
		client.CreateBrokerOrFail(brokerName, resources.WithChannelTemplateForBroker(&channel))

		t.Log("Wait for all resources to become ready")
		client.WaitForAllTestResourcesReadyOrFail()

		for _, e := range events {
			t.Log("send event", e)
			client.SendFakeEventToAddressableOrFail(senderName+e.ID, brokerName, lib.BrokerTypeMeta, e)
		}

		t.Log("Check logs", ids)
		if err := client.CheckLog(dropLoggerPodName, lib.CheckerContainsAll(ids)); err != nil {
			t.Fatalf("Strings %v not found in logs of logger pod %q: %v", ids, dropLoggerPodName, err)
		}
	})
}

func events(n uint8, source, t string) ([]*cloudevents.CloudEvent, []string) {
	events := make([]*cloudevents.CloudEvent, n)
	ids := make([]string, n)
	for i := range events {
		id := string(uuid.NewUUID())
		ids[i] = id
		events[i] = cloudevents.New(
			fmt.Sprintf(`{%q:%q}`, "msg", id),
			cloudevents.WithID(id),
			cloudevents.WithSource(source),
			cloudevents.WithType(t),
		)
	}
	return events, ids
}

func pod(imageName, name string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: v1.ObjectMeta{
			Name:   name,
			Labels: map[string]string{"e2etest": string(uuid.NewUUID())},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Name:            imageName,
				Image:           pkgTest.ImagePath(imageName),
				ImagePullPolicy: corev1.PullAlways,
			}},
			RestartPolicy: corev1.RestartPolicyNever,
		},
	}
}
