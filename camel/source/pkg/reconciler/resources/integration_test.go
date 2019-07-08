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

	"github.com/knative/eventing-contrib/camel/source/pkg/apis/sources/v1alpha1"

	camelv1alpha1 "github.com/apache/camel-k/pkg/apis/camel/v1alpha1"
	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMakeDeployment_sink(t *testing.T) {
	got, err := MakeIntegration(&CamelArguments{
		Name:      "test-name",
		Namespace: "test-namespace",
		Source: v1alpha1.CamelSourceOriginSpec{
			Component: &v1alpha1.CamelSourceOriginComponentSpec{
				URI: "timer:tick",
				Properties: map[string]string{
					"k":  "v",
					"k2": "v2",
				},
				ServiceAccountName: "test-service-account",
				Context:            "test-context",
			},
		},
		Sink: "http://test-sink",
	})
	if err != nil {
		t.Error(err)
	}

	want := &camelv1alpha1.Integration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "camel.apache.org/v1alpha1",
			Kind:       "Integration",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "test-name-",
			Namespace:    "test-namespace",
		},
		Spec: camelv1alpha1.IntegrationSpec{
			ServiceAccountName: "test-service-account",
			Context:            "test-context",
			Sources: []camelv1alpha1.SourceSpec{
				{
					DataSpec: camelv1alpha1.DataSpec{
						Name:    "source.flow",
						Content: "- steps:\n  - kind: endpoint\n    uri: timer:tick\n  - kind: endpoint\n    uri: knative:endpoint/sink\n",
					},
				},
			},
			Configuration: []camelv1alpha1.ConfigurationSpec{
				{
					Type:  "property",
					Value: "k=v",
				},
				{
					Type:  "property",
					Value: "k2=v2",
				},
			},
			Traits: map[string]camelv1alpha1.TraitSpec{
				"knative": {
					Configuration: map[string]string{
						"configuration": `{"services":[{"type":"endpoint","protocol":"http","name":"sink","host":"test-sink","port":80,"metadata":{"service.path":"/"}}]}`,
					},
				},
			},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected integration (-want, +got) = %v", diff)
	}
}
