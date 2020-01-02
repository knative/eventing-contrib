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
	meta "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing-contrib/camel/source/pkg/apis/sources/v1alpha1"
	camelsourceclient "knative.dev/eventing-contrib/camel/source/pkg/client/clientset/versioned"
	"knative.dev/eventing/test/base/resources"
	"knative.dev/eventing/test/common"
	knativeduck "knative.dev/pkg/apis/duck/v1beta1"
	"testing"
	"time"
)

func createCamelSourceOrFail(c *common.Client, camelSource *v1alpha1.CamelSource) {
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

func TestCamelSource(t *testing.T) {

	const (
		camelSourceName = "e2e-camelsource"
		loggerPodName = "e2e-camelsource-logger-pod"
		body = "Hello, world!"
	)

	client := common.Setup(t, true)
	defer common.TearDown(client)

	t.Logf("Creating logger Pod")
	pod := resources.EventLoggerPod(loggerPodName)
	client.CreatePodOrFail(pod, common.WithService(loggerPodName))

	t.Logf("Creating CamelSource")
	createCamelSourceOrFail(client, &v1alpha1.CamelSource{
		ObjectMeta: meta.ObjectMeta{
			Name: camelSourceName,
		},
		Spec: v1alpha1.CamelSourceSpec{
			Source: v1alpha1.CamelSourceOriginSpec{
				Flow: &v1alpha1.Flow{
					"from": &map[string]interface{} {
						"uri": "timer:tick?period=1s",
						"steps": []interface{} {
							&map[string]interface{} {
								"set-body": &map[string]interface{} {
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

	if err := client.CheckLog(loggerPodName, common.CheckerContains(body)); err != nil {
		t.Fatalf("Strings %q not found in logs of logger pod %q: %v", body, loggerPodName, err)
	}
}
