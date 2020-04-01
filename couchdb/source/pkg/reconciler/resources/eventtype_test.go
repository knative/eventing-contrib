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
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	eventingv1alpha1 "knative.dev/eventing/pkg/apis/eventing/v1alpha1"
	"knative.dev/eventing/pkg/utils"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmp"

	"knative.dev/eventing-contrib/couchdb/source/pkg/apis/sources/v1alpha1"
)

func TestMakeEventType(t *testing.T) {
	eventSrc := &EventTypeArgs{
		Src: &v1alpha1.CouchDbSource{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "source-test-namespace",
				Name:      "source-test-name-",
			},
			Spec: v1alpha1.CouchDbSourceSpec{
				ServiceAccountName: "test-service-account",
				Sink: &duckv1.Destination{
					Ref: &duckv1.KReference{
						Name:       "test-sink",
						Kind:       "Sink",
						APIVersion: "duck.knative.dev/v1",
					},
				},
			},
		},
		Type:   "test-type",
		Source: "test-source",
	}

	result := MakeEventType(eventSrc)
	want := &eventingv1alpha1.EventType{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", utils.ToDNS1123Subdomain("test-type")),
			Labels:       Labels("source-test-name-"),
			Namespace:    "source-test-namespace",
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(eventSrc.Src, schema.GroupVersionKind{
					Group:   v1alpha1.SchemeGroupVersion.Group,
					Version: v1alpha1.SchemeGroupVersion.Version,
					Kind:    "KafkaSource",
				}),
			},
		},
		Spec: eventingv1alpha1.EventTypeSpec{
			Type:   "type-test",
			Source: "test-source",
			Broker: "test-sink",
		},
	}
	if diff, err := kmp.SafeDiff(result, want); err != nil {
		t.Errorf("Unexpected eventtype: (-want, +got) = %v", diff)
	}
}
