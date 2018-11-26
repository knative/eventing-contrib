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

package v1alpha1

import (
	"testing"

	"github.com/google/go-cmp/cmp/cmpopts"

	"github.com/google/go-cmp/cmp"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func TestKubernetesEventSourceStatusIsReady(t *testing.T) {
	tests := []struct {
		name string
		s    *KubernetesEventSourceStatus
		want bool
	}{{
		name: "uninitialized",
		s:    &KubernetesEventSourceStatus{},
		want: false,
	}, {
		name: "initialized",
		s: func() *KubernetesEventSourceStatus {
			s := &KubernetesEventSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		want: false,
	}, {
		name: "mark ready",
		s: func() *KubernetesEventSourceStatus {
			s := &KubernetesEventSourceStatus{}
			s.InitializeConditions()
			s.MarkReady()
			return s
		}(),
		want: true,
	}, {
		name: "mark ready then unready",
		s: func() *KubernetesEventSourceStatus {
			s := &KubernetesEventSourceStatus{}
			s.InitializeConditions()
			s.MarkReady()
			s.MarkUnready("Testing", "")
			return s
		}(),
		want: false,
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.s.IsReady()
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("%s: unexpected condition (-want, +got) = %v", test.name, diff)
			}
		})
	}
}

func TestKubernetesEventSourceStatusGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		s         *KubernetesEventSourceStatus
		condQuery duckv1alpha1.ConditionType
		want      *duckv1alpha1.Condition
	}{{
		name:      "uninitialized",
		s:         &KubernetesEventSourceStatus{},
		condQuery: KubernetesEventSourceConditionReady,
		want:      nil,
	}, {
		name: "initialized",
		s: func() *KubernetesEventSourceStatus {
			s := &KubernetesEventSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		condQuery: KubernetesEventSourceConditionReady,
		want: &duckv1alpha1.Condition{
			Type:   KubernetesEventSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark ready",
		s: func() *KubernetesEventSourceStatus {
			s := &KubernetesEventSourceStatus{}
			s.InitializeConditions()
			s.MarkReady()
			return s
		}(),
		condQuery: KubernetesEventSourceConditionReady,
		want: &duckv1alpha1.Condition{
			Type:   KubernetesEventSourceConditionReady,
			Status: corev1.ConditionTrue,
		},
	}, {
		name: "mark ready then unready",
		s: func() *KubernetesEventSourceStatus {
			s := &KubernetesEventSourceStatus{}
			s.InitializeConditions()
			s.MarkReady()
			s.MarkUnready("Testing", "hi")
			return s
		}(),
		condQuery: KubernetesEventSourceConditionReady,
		want: &duckv1alpha1.Condition{
			Type:    KubernetesEventSourceConditionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "Testing",
			Message: "hi",
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.s.GetCondition(test.condQuery)
			ignoreTime := cmpopts.IgnoreFields(duckv1alpha1.Condition{},
				"LastTransitionTime", "Severity")
			if diff := cmp.Diff(test.want, got, ignoreTime); diff != "" {
				t.Errorf("unexpected condition (-want, +got) = %v", diff)
			}
		})
	}
}
