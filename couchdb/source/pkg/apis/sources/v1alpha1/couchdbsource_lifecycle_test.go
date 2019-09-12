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
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

var condReady = apis.Condition{
	Type:   CouchDbConditionReady,
	Status: corev1.ConditionTrue,
}

func TestCouchDbGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		cs        *CouchDbSourceStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name: "single condition",
		cs: &CouchDbSourceStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{
					condReady,
				},
			},
		},
		condQuery: apis.ConditionReady,
		want:      &condReady,
	}, {
		name: "unknown condition",
		cs: &CouchDbSourceStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{
					condReady,
				},
			},
		},
		condQuery: apis.ConditionType("foo"),
		want:      nil,
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.cs.GetCondition(test.condQuery)
			if diff := cmp.Diff(test.want, got); diff != "" {
				t.Errorf("unexpected condition (-want, +got) = %v", diff)
			}
		})
	}
}

func TestCouchDbInitializeConditions(t *testing.T) {
	tests := []struct {
		name string
		cs   *CouchDbSourceStatus
		want *CouchDbSourceStatus
	}{{
		name: "empty",
		cs:   &CouchDbSourceStatus{},
		want: &CouchDbSourceStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
					Type:   CouchDbConditionDeployed,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   CouchDbConditionEventTypeProvided,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   CouchDbConditionReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   CouchDbConditionSinkProvided,
					Status: corev1.ConditionUnknown,
				}},
			},
		},
	}, {
		name: "one false",
		cs: &CouchDbSourceStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
					Type:   CouchDbConditionSinkProvided,
					Status: corev1.ConditionFalse,
				}},
			},
		},
		want: &CouchDbSourceStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
					Type:   CouchDbConditionDeployed,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   CouchDbConditionEventTypeProvided,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   CouchDbConditionReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   CouchDbConditionSinkProvided,
					Status: corev1.ConditionFalse,
				}},
			},
		},
	}, {
		name: "one true",
		cs: &CouchDbSourceStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
					Type:   CouchDbConditionSinkProvided,
					Status: corev1.ConditionTrue,
				}},
			},
		},
		want: &CouchDbSourceStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
					Type:   CouchDbConditionDeployed,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   CouchDbConditionEventTypeProvided,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   CouchDbConditionReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   CouchDbConditionSinkProvided,
					Status: corev1.ConditionTrue,
				}},
			},
		},
	}, {
		name: "marksink",
		cs: func() *CouchDbSourceStatus {
			status := CouchDbSourceStatus{}
			status.MarkSink("http://sink")
			return &status
		}(),
		want: &CouchDbSourceStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
					Type:   CouchDbConditionDeployed,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   CouchDbConditionEventTypeProvided,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   CouchDbConditionReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   CouchDbConditionSinkProvided,
					Status: corev1.ConditionTrue,
				}},
			},
			SinkURI: "http://sink",
		},
	}, {
		name: "marknosink",
		cs: func() *CouchDbSourceStatus {
			status := CouchDbSourceStatus{}
			status.MarkNoSink("nothere", "")
			return &status
		}(),
		want: &CouchDbSourceStatus{
			Status: duckv1beta1.Status{
				Conditions: []apis.Condition{{
					Type:   CouchDbConditionDeployed,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   CouchDbConditionEventTypeProvided,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   CouchDbConditionReady,
					Status: corev1.ConditionFalse,
				}, {
					Type:   CouchDbConditionSinkProvided,
					Status: corev1.ConditionFalse,
				}},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			test.cs.InitializeConditions()
			ignore := cmpopts.IgnoreFields(
				apis.Condition{},
				"LastTransitionTime", "Message", "Reason", "Severity")
			if diff := cmp.Diff(test.want, test.cs, ignore); diff != "" {
				t.Errorf("unexpected conditions (-want, +got) = %v", diff)
			}
		})
	}
}
