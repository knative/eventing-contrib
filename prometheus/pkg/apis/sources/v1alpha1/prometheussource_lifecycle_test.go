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
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (
	availableDeployment = &appsv1.Deployment{
		Status: appsv1.DeploymentStatus{
			Conditions: []appsv1.DeploymentCondition{
				{
					Type:   appsv1.DeploymentAvailable,
					Status: corev1.ConditionTrue,
				},
			},
		},
	}

	condReady = apis.Condition{
		Type:   PrometheusConditionReady,
		Status: corev1.ConditionTrue,
	}
)

func TestPrometheusGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		cs        *PrometheusSourceStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name: "single condition",
		cs: &PrometheusSourceStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					condReady,
				},
			},
		},
		condQuery: apis.ConditionReady,
		want:      &condReady,
	}, {
		name: "unknown condition",
		cs: &PrometheusSourceStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{
					condReady,
				},
			},
		},
		condQuery: apis.ConditionType("foo"),
		want:      nil,
	}, {
		name: "mark deployed",
		cs: func() *PrometheusSourceStatus {
			s := &PrometheusSourceStatus{}
			s.InitializeConditions()
			s.PropagateDeploymentAvailability(availableDeployment)
			return s
		}(),
		condQuery: PrometheusConditionReady,
		want: &apis.Condition{
			Type:   PrometheusConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark sink and deployed and event types",
		cs: func() *PrometheusSourceStatus {
			s := &PrometheusSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.PropagateDeploymentAvailability(availableDeployment)
			s.MarkEventTypes()
			return s
		}(),
		condQuery: PrometheusConditionReady,
		want: &apis.Condition{
			Type:   PrometheusConditionReady,
			Status: corev1.ConditionTrue,
		},
	}}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.cs.GetCondition(test.condQuery)
			ignoreTime := cmpopts.IgnoreFields(apis.Condition{},
				"LastTransitionTime", "Severity")
			if diff := cmp.Diff(test.want, got, ignoreTime); diff != "" {
				t.Errorf("unexpected condition (-want, +got) = %v", diff)
			}
		})
	}
}

func TestPrometheusInitializeConditions(t *testing.T) {
	tests := []struct {
		name string
		cs   *PrometheusSourceStatus
		want *PrometheusSourceStatus
	}{{
		name: "empty",
		cs:   &PrometheusSourceStatus{},
		want: &PrometheusSourceStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   PrometheusConditionDeployed,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   PrometheusConditionEventTypeProvided,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   PrometheusConditionReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   PrometheusConditionSinkProvided,
					Status: corev1.ConditionUnknown,
				}},
			},
		},
	}, {
		name: "one false",
		cs: &PrometheusSourceStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   PrometheusConditionSinkProvided,
					Status: corev1.ConditionFalse,
				}},
			},
		},
		want: &PrometheusSourceStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   PrometheusConditionDeployed,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   PrometheusConditionEventTypeProvided,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   PrometheusConditionReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   PrometheusConditionSinkProvided,
					Status: corev1.ConditionFalse,
				}},
			},
		},
	}, {
		name: "one true",
		cs: &PrometheusSourceStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   PrometheusConditionSinkProvided,
					Status: corev1.ConditionTrue,
				}},
			},
		},
		want: &PrometheusSourceStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   PrometheusConditionDeployed,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   PrometheusConditionEventTypeProvided,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   PrometheusConditionReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   PrometheusConditionSinkProvided,
					Status: corev1.ConditionTrue,
				}},
			},
		},
	}, {
		name: "marksink",
		cs: func() *PrometheusSourceStatus {
			status := PrometheusSourceStatus{}
			status.MarkSink("http://sink")
			return &status
		}(),
		want: &PrometheusSourceStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   PrometheusConditionDeployed,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   PrometheusConditionEventTypeProvided,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   PrometheusConditionReady,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   PrometheusConditionSinkProvided,
					Status: corev1.ConditionTrue,
				}},
			},
			SinkURI: "http://sink",
		},
	}, {
		name: "marknosink",
		cs: func() *PrometheusSourceStatus {
			status := PrometheusSourceStatus{}
			status.MarkNoSink("nothere", "")
			return &status
		}(),
		want: &PrometheusSourceStatus{
			Status: duckv1.Status{
				Conditions: []apis.Condition{{
					Type:   PrometheusConditionDeployed,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   PrometheusConditionEventTypeProvided,
					Status: corev1.ConditionUnknown,
				}, {
					Type:   PrometheusConditionReady,
					Status: corev1.ConditionFalse,
				}, {
					Type:   PrometheusConditionSinkProvided,
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
