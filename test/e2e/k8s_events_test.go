// +build e2e

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
	"time"

	"github.com/knative/eventing-sources/test"
	pkgTest "github.com/knative/pkg/test"
)

const (
	serviceAccount   = "e2e-receive-adapter"
	eventSource      = "k8sevents"
	routeName        = "e2e-k8s-events-function"
	channelName      = "e2e-k8s-events-channel"
	provisionerName  = "in-memory-channel"
	subscriptionName = "e2e-k8s-events-subscription"
)

func TestKubernetesEvents(t *testing.T) {

	// The plan is to remove k8s source. This e2e test is flaky. Works in non-prow cluster.
	t.Skip()

	clients, cleaner := Setup(t, t.Logf)

	pkgTest.CleanupOnInterrupt(func() { TearDown(clients, cleaner, t.Logf) }, t.Logf)
	defer TearDown(clients, cleaner, t.Logf)

	t.Log("Creating ServiceAccount and Binding")

	err := CreateServiceAccountAndBinding(clients, serviceAccount, t.Logf, cleaner)
	if err != nil {
		t.Fatalf("Failed to create ServiceAccount or Binding: %v", err)
	}

	t.Logf("Creating Channel")
	channel := test.Channel(channelName, pkgTest.Flags.Namespace, test.ClusterChannelProvisioner(provisionerName))
	err = CreateChannel(clients, channel, t.Logf, cleaner)
	if err != nil {
		t.Fatalf("Failed to create Channel: %v", err)
	}

	t.Logf("Creating EventSource")
	k8sSource := test.KubernetesEventSource(eventSource, pkgTest.Flags.Namespace, testNamespace, serviceAccount, test.ChannelRef(channelName))
	err = CreateKubernetesEventSource(clients, k8sSource, t.Logf, cleaner)
	if err != nil {
		t.Fatalf("Failed to create KubernetesEventSource: %v", err)
	}

	t.Logf("Creating Route and Config")
	// The receiver of events which is accessible through Route
	configImagePath := pkgTest.ImagePath("k8sevents")
	err = WithRouteReady(clients, t.Logf, cleaner, routeName, configImagePath)
	if err != nil {
		t.Fatalf("The Route was not marked as Ready to serve traffic: %v", err)
	}

	t.Logf("Creating Subscription")
	subscription := test.Subscription(subscriptionName, pkgTest.Flags.Namespace, test.ChannelRef(channelName), test.SubscriberSpecForRoute(routeName), nil)
	err = CreateSubscription(clients, subscription, t.Logf, cleaner)
	if err != nil {
		t.Fatalf("Failed to create Subscription: %v", err)
	}

	//Work around for: https://github.com/knative/eventing/issues/125
	//and the fact that even after pods are up, due to Istio slowdown, there's
	//about 5-6 seconds that traffic won't be passed through.
	pkgTest.WaitForAllPodsRunning(clients.Kube, pkgTest.Flags.Namespace)
	time.Sleep(10 * time.Second)

	t.Logf("Creating Pod")

	err = CreatePod(clients, pkgTest.NginxPod(testNamespace), t.Logf, cleaner)
	if err != nil {
		t.Fatalf("Failed to create Pod: %v", err)
	}

	pkgTest.WaitForAllPodsRunning(clients.Kube, testNamespace)

	err = pkgTest.WaitForLogContent(clients.Kube, routeName, "user-container", testNamespace, "Created container")
	if err != nil {
		t.Fatalf("Events for container created not received: %v", err)
	}
	err = pkgTest.WaitForLogContent(clients.Kube, routeName, "user-container", testNamespace, "Started container")
	if err != nil {
		t.Fatalf("Events for container started not received: %v", err)
	}
}
