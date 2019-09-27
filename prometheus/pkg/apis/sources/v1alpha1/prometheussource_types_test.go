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

package v1alpha1

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

func TestPrometheusSourceStatusIsReady(t *testing.T) {
	tests := []struct {
		name string
		s    *PrometheusSourceStatus
		want bool
	}{{
		name: "uninitialized",
		s:    &PrometheusSourceStatus{},
		want: false,
	}, {
		name: "initialized",
		s: func() *PrometheusSourceStatus {
			s := &PrometheusSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		want: false,
	}, {
		name: "mark sink",
		s: func() *PrometheusSourceStatus {
			s := &PrometheusSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
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

func TestPrometheusSourceStatusGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		s         *PrometheusSourceStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name:      "uninitialized",
		s:         &PrometheusSourceStatus{},
		condQuery: PrometheusConditionReady,
		want:      nil,
	}, {
		name: "initialized",
		s: func() *PrometheusSourceStatus {
			s := &PrometheusSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		condQuery: PrometheusConditionReady,
		want: &apis.Condition{
			Type:   PrometheusConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark sink",
		s: func() *PrometheusSourceStatus {
			s := &PrometheusSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			return s
		}(),
		condQuery: PrometheusConditionReady,
		want: &apis.Condition{
			Type:   PrometheusConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.s.GetCondition(test.condQuery)
			ignoreTime := cmpopts.IgnoreFields(apis.Condition{},
				"LastTransitionTime", "Severity")
			if diff := cmp.Diff(test.want, got, ignoreTime); diff != "" {
				t.Errorf("unexpected condition (-want, +got) = %v", diff)
			}
		})
	}
}

func TestGVK(t *testing.T) {
	wantGVK := schema.GroupVersionKind{
		Group:   SchemeGroupVersion.Group,
		Version: SchemeGroupVersion.Version,
		Kind:    "PrometheusSource",
	}
	var ps PrometheusSource
	if diff := cmp.Diff(wantGVK, ps.GetGroupVersionKind()); diff != "" {
		t.Errorf("unexpected condition (-want, +got) = %v", diff)
	}
}
