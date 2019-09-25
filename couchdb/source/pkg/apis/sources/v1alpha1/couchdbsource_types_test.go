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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

func TestCouchDbSourceStatusIsReady(t *testing.T) {
	tests := []struct {
		name string
		s    *CouchDbSourceStatus
		want bool
	}{{
		name: "uninitialized",
		s:    &CouchDbSourceStatus{},
		want: false,
	}, {
		name: "initialized",
		s: func() *CouchDbSourceStatus {
			s := &CouchDbSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		want: false,
	}, {
		name: "mark sink",
		s: func() *CouchDbSourceStatus {
			s := &CouchDbSourceStatus{}
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

func TestCouchDbSourceStatusGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		s         *CouchDbSourceStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name:      "uninitialized",
		s:         &CouchDbSourceStatus{},
		condQuery: CouchDbConditionReady,
		want:      nil,
	}, {
		name: "initialized",
		s: func() *CouchDbSourceStatus {
			s := &CouchDbSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		condQuery: CouchDbConditionReady,
		want: &apis.Condition{
			Type:   CouchDbConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark sink",
		s: func() *CouchDbSourceStatus {
			s := &CouchDbSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			return s
		}(),
		condQuery: CouchDbConditionReady,
		want: &apis.Condition{
			Type:   CouchDbConditionReady,
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
