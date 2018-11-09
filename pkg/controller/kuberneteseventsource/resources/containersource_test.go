/*
Copyright 2018 The Knative Authors

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

	"github.com/google/go-cmp/cmp"
	sourcesv1alpha1 "github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMakeContainerSource(t *testing.T) {
	got := MakeContainerSource(&sourcesv1alpha1.KubernetesEventSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kes",
			Namespace: "ns",
		},
		Spec: sourcesv1alpha1.KubernetesEventSourceSpec{
			ServiceAccountName: "serviceaccount",
			Namespace:          "watchns",
			Sink: &corev1.ObjectReference{
				Name:      "sink",
				Namespace: "sinkns",
			},
		},
	}, "raimage")

	want := &sourcesv1alpha1.ContainerSource{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "kes-",
			Namespace:    "ns",
		},
		Spec: sourcesv1alpha1.ContainerSourceSpec{
			Image:              "raimage",
			Args:               []string{"--namespace=watchns"},
			ServiceAccountName: "serviceaccount",
			Sink: &corev1.ObjectReference{
				Name:      "sink",
				Namespace: "sinkns",
			},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("Unexpected difference: (-want, +got): %v", diff)
	}

}
