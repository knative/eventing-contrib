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

package resources

import (
	"testing"

	"knative.dev/eventing-contrib/camel/source/pkg/apis/sources/v1alpha1"

	camelv1 "github.com/apache/camel-k/pkg/apis/camel/v1"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMakeDeployment_sink(t *testing.T) {
	got, err := MakeIntegration(&CamelArguments{
		Name:      "test-name",
		Namespace: "test-namespace",
		Source: v1alpha1.CamelSourceOriginSpec{
			Flow: &v1alpha1.Flow{
				"from": map[string]interface{}{
					"uri": "timer:tick",
				},
			},
			Integration: &camelv1.IntegrationSpec{
				ServiceAccountName: "test-service-account",
				Kit:                "test-kit",
				Configuration: []camelv1.ConfigurationSpec{
					{
						Type:  "property",
						Value: "k=v",
					},
					{
						Type:  "property",
						Value: "k2=v2",
					},
				},
			},
		},
		SinkURL: "http://test-sink",
		Overrides: map[string]string{
			"a": "b",
		},
	})
	if err != nil {
		t.Error(err)
	}

	want := &camelv1.Integration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "camel.apache.org/v1",
			Kind:       "Integration",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-name-",
			Namespace:    "test-namespace",
		},
		Spec: camelv1.IntegrationSpec{
			ServiceAccountName: "test-service-account",
			Kit:                "test-kit",
			Sources: []camelv1.SourceSpec{
				{
					Loader: "knative-source",
					DataSpec: camelv1.DataSpec{
						Name:    "flow.yaml",
						Content: "- from:\n    uri: timer:tick\n",
					},
				},
			},
			Configuration: []camelv1.ConfigurationSpec{
				{
					Type:  "property",
					Value: "k=v",
				},
				{
					Type:  "property",
					Value: "k2=v2",
				},
			},
			Traits: map[string]camelv1.TraitSpec{
				"knative": {
					Configuration: map[string]string{
						"configuration": `{"services":[{"type":"endpoint","name":"sink","host":"test-sink","port":80,"metadata":{"camel.endpoint.kind":"sink","ce.override.ce-a":"b","ce.override.ce-source":"camel-source:test-namespace/test-name","knative.apiVersion":"","knative.kind":"","service.path":"/"}}]}`,
					},
				},
			},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected integration (-want, +got) = %v", diff)
	}
}
