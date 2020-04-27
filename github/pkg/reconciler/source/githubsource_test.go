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

package source

import (
	"os"
	"reflect"
	"testing"

	"knative.dev/eventing-contrib/github/pkg/reconciler/source/resources"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	"knative.dev/pkg/reconciler"
	rectesting "knative.dev/pkg/reconciler/testing"
	"knative.dev/pkg/resolver"
	"knative.dev/pkg/tracker"
	servingv1 "knative.dev/serving/pkg/apis/serving/v1"
	fakeservingclient "knative.dev/serving/pkg/client/injection/client/fake"
	servingtesting "knative.dev/serving/pkg/reconciler/testing/v1"

	"knative.dev/eventing-contrib/github/pkg/apis/sources/v1alpha1"
	. "knative.dev/eventing-contrib/test/lib/resources"
)

const (
	testImage       = "test-image"
	testName        = "test-name"
	testNs          = "test-ns"
	secretName      = "test-secret"
	secretKey       = "test-secret-key"
	accessTokenName = "test-access-token"
	accessTokenKey  = "test-access-token-key"
	controllerName  = "github-controller"
	controllerUID   = "543892"
)

var (
	sinkName = "test-sink"
	sinkDest = duckv1.Destination{
		Ref: &duckv1.KReference{
			Name:       sinkName,
			Namespace:  testNs,
			Kind:       "Service",
			APIVersion: "v1",
		},
	}
	sinkDNS    = "test-host"
	sinkURL    = apis.URL{Host: sinkDNS}
	adapterURL = apis.URL{Host: "test-adapter"}
)

func TestGitHubSource(t *testing.T) {
	os.Setenv("CONTROLLER_NAME", controllerName)
	os.Setenv("CONTROLLER_UID", controllerUID)

	testCases := []struct {
		name           string
		kubeObjects    []runtime.Object
		servingObjects []runtime.Object
		source         *v1alpha1.GitHubSource
		expectEvent    reconciler.Event
	}{
		{
			name: "valid",
			source: NewGitHubSourceV1Alpha1(testName, testNs,
				WithGitHubSourceSpecV1Alpha1(v1alpha1.GitHubSourceSpec{
					SecretToken: NewSecretValueFromSource(secretName, secretKey),
					AccessToken: NewSecretValueFromSource(accessTokenName, accessTokenKey),
					Sink:        &sinkDest,
				}),
				WithGithubSourceConditionsV1Alpha1,
				WithGithubSourceSecretV1Alpha1,
				WithGithubSourceWebHookConfiguredV1Alpha1,
				WithGithubSourceSinkV1Alpha1(&sinkURL)),
			kubeObjects: []runtime.Object{
				newSecret(secretName, secretKey),
				newSecret(accessTokenName, accessTokenKey),
				newService(sinkName),
			},
			servingObjects: []runtime.Object{
				makeAvailableAdapter(),
			},
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := rectesting.SetupFakeContext(t)
			ctx, kubeClient := fakekubeclient.With(ctx, test.kubeObjects...)
			ctx, servingClient := fakeservingclient.With(ctx)
			listers := servingtesting.NewListers(test.servingObjects)

			reconciler := Reconciler{
				kubeClientSet:       kubeClient,
				servingClientSet:    servingClient,
				servingLister:       listers.GetServiceLister(),
				receiveAdapterImage: testImage,
				tracker:             tracker.New(func(types.NamespacedName) {}, 0),
				sinkResolver:        resolver.NewURIResolver(ctx, func(types.NamespacedName) {}),
				webhookClient:       NewFakeWebhookClient(),
			}

			event := reconciler.ReconcileKind(ctx, test.source)

			if !reflect.DeepEqual(event, test.expectEvent) {
				t.Errorf("Unexpected event Want %v, got %v", test.expectEvent, event)
			}

			reconciler.FinalizeKind(ctx, test.source)
		})
	}
}

func newSecret(name, key string) *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNs,
		},
		Data: map[string][]byte{
			key: []byte("hfiegi"),
		},
	}
}

func newService(name string) *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: testNs,
		},
	}
}

func makeAvailableAdapter() *servingv1.Service {
	adapter := resources.MakeService(&resources.ServiceArgs{
		ServiceAccountName:  saName,
		ReceiveAdapterImage: testImage,
		AdapterName:         adapterName,
	})
	adapter.Status = servingv1.ServiceStatus{
		Status: duckv1.Status{
			Conditions: duckv1.Conditions{
				{
					Type:   servingv1.ServiceConditionReady,
					Status: corev1.ConditionTrue,
				},
			},
		},
		RouteStatusFields: servingv1.RouteStatusFields{
			URL: &adapterURL,
		},
	}
	return adapter
}
