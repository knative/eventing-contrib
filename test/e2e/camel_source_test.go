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
	"time"

	camelv1 "github.com/apache/camel-k/pkg/apis/camel/v1"
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"knative.dev/eventing-contrib/camel/source/pkg/apis/sources/v1alpha1"
	camelsourceclient "knative.dev/eventing-contrib/camel/source/pkg/client/clientset/versioned"
	"knative.dev/eventing/test/lib"
	"knative.dev/eventing/test/lib/resources"
	knativeduck "knative.dev/pkg/apis/duck/v1beta1"
	runtime "sigs.k8s.io/controller-runtime/pkg/client"
)

func TestCamelSource(t *testing.T) {

	const (
		camelSourceName = "e2e-camelsource"
		loggerPodName   = "e2e-camelsource-logger-pod"
		body            = "Hello, world!"
	)

	client := lib.Setup(t, true)
	defer lib.TearDown(client)

	t.Logf("Creating logger Pod")
	pod := resources.EventLoggerPod(loggerPodName)
	client.CreatePodOrFail(pod, lib.WithService(loggerPodName))

	camelClient := getCamelKClient(client)

	t.Logf("Creating Camel K IntegrationPlatform")
	createCamelPlatformOrFail(client, camelClient, camelSourceName)

	t.Logf("Creating Camel K Kit (to skip build)")
	createCamelKitOrFail(client, camelClient, camelSourceName)

	t.Logf("Creating CamelSource")
	createCamelSourceOrFail(client, &v1alpha1.CamelSource{
		ObjectMeta: meta.ObjectMeta{
			Name: camelSourceName,
		},
		Spec: v1alpha1.CamelSourceSpec{
			Source: v1alpha1.CamelSourceOriginSpec{
				Flow: &v1alpha1.Flow{
					"from": &map[string]interface{}{
						"uri": "timer:tick?period=1s",
						"steps": []interface{}{
							&map[string]interface{}{
								"set-body": &map[string]interface{}{
									"constant": body,
								},
							},
						},
					},
				},
			},
			Sink: &knativeduck.Destination{
				Ref: resources.ServiceRef(loggerPodName),
			},
		},
	})

	t.Logf("Waiting for all resources ready")
	client.WaitForAllTestResourcesReadyOrFail()

	t.Logf("Sleeping for 3s to let the timer tick at least once")
	time.Sleep(3 * time.Second)

	pods, err := client.Kube.Kube.CoreV1().Pods(client.Namespace).List(meta.ListOptions{
		LabelSelector: "camel.apache.org/integration",
	})
	if err != nil {
		t.Fatalf("cannot get integration pod: %v", err)
	}
	if len(pods.Items) == 0 {
		t.Fatalf("no integration pod found")
	}
	printPodLogs(t, client, pods.Items[0].Name, "integration")

	if err := client.CheckLog(loggerPodName, lib.CheckerContains(body)); err != nil {
		printPodLogs(t, client, pods.Items[0].Name, "integration")
		t.Fatalf("Strings %q not found in logs of logger pod %q: %v", body, loggerPodName, err)
	}
}

func printPodLogs(t *testing.T, c *lib.Client, podName, containerName string) {
	logs, err := c.Kube.PodLogs(podName, containerName, c.Namespace)
	if err == nil {
		t.Log(string(logs))
	}
	t.Logf("End of pod %s logs", podName)
}

func createCamelSourceOrFail(c *lib.Client, camelSource *v1alpha1.CamelSource) {
	camelSourceClientSet, err := camelsourceclient.NewForConfig(c.Config)
	if err != nil {
		c.T.Fatalf("Failed to create CamelSource client: %v", err)
	}

	cSources := camelSourceClientSet.SourcesV1alpha1().CamelSources(c.Namespace)
	if createdCamelSource, err := cSources.Create(camelSource); err != nil {
		c.T.Fatalf("Failed to create CamelSource %q: %v", camelSource.Name, err)
	} else {
		c.Tracker.AddObj(createdCamelSource)
	}
}

func createCamelPlatformOrFail(c *lib.Client, camelClient runtime.Client, camelSourceName string) {
	platform := camelv1.IntegrationPlatform{
		ObjectMeta: meta.ObjectMeta{
			Name:      "camel-k",
			Namespace: c.Namespace,
		},
		Spec: camelv1.IntegrationPlatformSpec{
			Profile: camelv1.TraitProfileKnative,
		},
	}

	if err := camelClient.Create(context.TODO(), &platform); err != nil {
		c.T.Fatalf("Failed to create IntegrationPlatform for CamelSource %q: %v", camelSourceName, err)
	}
}

func createCamelKitOrFail(c *lib.Client, camelClient runtime.Client, camelSourceName string) {
	// Creating this kit manually because the Camel K platform is not configured to do it on its own.
	// Testing that Camel K works is not in scope for this test.
	kit := camelv1.IntegrationKit{
		ObjectMeta: meta.ObjectMeta{
			Name:      "test-kit",
			Namespace: c.Namespace,
			Labels: map[string]string{
				"camel.apache.org/kit.type": "external",
			},
		},
		Spec: camelv1.IntegrationKitSpec{
			Dependencies: []string{
				"camel:timer",
				"mvn:org.apache.camel.k/camel-k-loader-knative",
				"mvn:org.apache.camel.k/camel-k-loader-yaml",
				"mvn:org.apache.camel.k/camel-k-runtime-knative",
				"mvn:org.apache.camel.k/camel-k-runtime-main",
			},
			Image: "docker.io/testcamelk/camel-k-kit-knative-timer:1.0.0-RC2",
		},
	}

	if err := camelClient.Create(context.TODO(), &kit); err != nil {
		c.T.Fatalf("Failed to create IntegrationKit for CamelSource %q: %v", camelSourceName, err)
	}
}

func getCamelKClient(c *lib.Client) runtime.Client {
	scheme := k8sruntime.NewScheme()
	scheme.AddKnownTypes(camelv1.SchemeGroupVersion, &camelv1.IntegrationPlatform{}, &camelv1.IntegrationPlatformList{})
	scheme.AddKnownTypes(camelv1.SchemeGroupVersion, &camelv1.IntegrationKit{}, &camelv1.IntegrationKitList{})
	meta.AddToGroupVersion(scheme, camelv1.SchemeGroupVersion)
	options := runtime.Options{
		Scheme: scheme,
	}
	client, err := runtime.New(c.Config, options)
	if err != nil {
		c.T.Fatalf("Failed to create initialize generic client for Camel K resources: %v", err)
	}
	return client
}
