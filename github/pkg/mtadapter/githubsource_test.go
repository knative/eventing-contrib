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

package mtadapter

import (
	"fmt"
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/eventing/pkg/logging"
	"knative.dev/pkg/apis"
	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	"knative.dev/pkg/reconciler"
	rectesting "knative.dev/pkg/reconciler/testing"

	"knative.dev/eventing-contrib/github/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing-contrib/test/lib/resources"
)

const (
	testName   = "test-name"
	testNs     = "test-ns"
	secretName = "test-secret"
	secretKey  = "test-secret-key"
)

var (
	testSink = apis.URL{Host: "test-host"}
)

func TestGitHubSource(t *testing.T) {
	testCases := []struct {
		name        string
		source      *v1alpha1.GitHubSource
		expectEvent reconciler.Event
	}{
		{
			name: "resource not ready valid",
			source: resources.NewGitHubSourceV1Alpha1(testName, testNs,
				resources.WithGitHubSourceSpecV1Alpha1(v1alpha1.GitHubSourceSpec{})),
			expectEvent: fmt.Errorf("GitHubSource is not ready. Cannot configure the adapter"),
		},
		{
			name: "valid",
			source: resources.NewGitHubSourceV1Alpha1(testName, testNs,
				resources.WithGitHubSourceSpecV1Alpha1(v1alpha1.GitHubSourceSpec{}),
				resources.WithGitHubSecretTokenSpecV1Alpha1(secretName, secretKey),
				resources.WithGithubSourceConditionsV1Alpha1,
				resources.WithGithubSourceSecretV1Alpha1,
				resources.WithGithubSourceWebHookConfiguredV1Alpha1,
				resources.WithGithubSourceSinkV1Alpha1(&testSink)),
		},
	}

	for _, test := range testCases {
		t.Run(test.name, func(t *testing.T) {
			ctx, _ := rectesting.SetupFakeContext(t)
			ctx, kubeClient := fakekubeclient.With(ctx, newSecret())

			reconciler := Reconciler{
				kubeClientSet: kubeClient,
				router:        NewRouter(logging.FromContext(ctx).Sugar(), nil, nil),
			}

			event := reconciler.ReconcileKind(ctx, test.source)

			if !reflect.DeepEqual(event, test.expectEvent) {
				t.Errorf("Unexpected event Want %v, got %v", test.expectEvent, event)
			}

		})
	}
}

func newSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      secretName,
			Namespace: testNs,
		},
		Data: map[string][]byte{
			secretKey: []byte("hfiegi"),
		},
	}
}
