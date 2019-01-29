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

package kubernetesevents

import (
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/pkg/cloudevents"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

func TestCloudEventOverrides(t *testing.T) {

	uid := types.UID("ABC")

	ref := corev1.ObjectReference{
		Kind:            "Kind",
		Namespace:       "Namespace",
		Name:            "Name",
		APIVersion:      "api/version",
		ResourceVersion: "v1test1",
		FieldPath:       "field",
	}
	refLink := "/apis/api/version/namespaces/Namespace/kinds/Name"

	now := time.Now()

	want := cloudevents.V01EventContext{
		EventID:   string(uid),
		Source:    refLink,
		EventTime: now,
	}

	event := &corev1.Event{
		ObjectMeta: metav1.ObjectMeta{
			UID:               uid,
			CreationTimestamp: metav1.Time{Time: now},
		},
		InvolvedObject: ref,
	}

	got := cloudEventOverrides(event)

	//ignoreTime := cmpopts.IgnoreFields(duckv1alpha1.Condition{}, "LastTransitionTime")
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected context (-want, +got) = %v", diff)
	}
}

func TestCreateSelfLink_nohack(t *testing.T) {
	expected := "/apis/api/version/namespaces/Namespace/kinds/Name"
	ref := corev1.ObjectReference{
		Kind:            "Kind",
		Namespace:       "Namespace",
		Name:            "Name",
		APIVersion:      "api/version",
		ResourceVersion: "v1test1",
		FieldPath:       "field",
	}

	got := createSelfLink(ref)

	if got != expected {
		t.Errorf("expected link to be %v, got %v", expected, got)
	}
}

func TestCreateSelfLink_hack(t *testing.T) {
	expected := "/apis/test.api/versionUnknown/namespaces/Namespace/kinds/Name"
	ref := corev1.ObjectReference{
		Kind:            "Kind",
		Namespace:       "Namespace",
		Name:            "Name",
		APIVersion:      "test.api",
		ResourceVersion: "v1test1",
		FieldPath:       "field",
	}

	got := createSelfLink(ref)

	if got != expected {
		t.Errorf("expected link to be %v, got %v", expected, got)
	}
}
