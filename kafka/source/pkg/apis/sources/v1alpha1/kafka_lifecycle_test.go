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
	"knative.dev/pkg/apis/duck"
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
		Type:   KafkaConditionReady,
		Status: corev1.ConditionTrue,
	}
)

// Check that KafkaSource implements the Conditions duck type.
var _ = duck.VerifyType(&KafkaSource{}, &duckv1.Conditions{})

func TestKafkaSourceStatusGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		s         *KafkaSourceStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name:      "uninitialized",
		s:         &KafkaSourceStatus{},
		condQuery: KafkaConditionReady,
		want:      nil,
	}, {
		name: "initialized",
		s: func() *KafkaSourceStatus {
			s := &KafkaSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		condQuery: KafkaConditionReady,
		want: &apis.Condition{
			Type:   KafkaConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark deployed",
		s: func() *KafkaSourceStatus {
			s := &KafkaSourceStatus{}
			s.InitializeConditions()
			s.MarkDeployed(availableDeployment)
			return s
		}(),
		condQuery: KafkaConditionReady,
		want: &apis.Condition{
			Type:   KafkaConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark sink",
		s: func() *KafkaSourceStatus {
			s := &KafkaSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			return s
		}(),
		condQuery: KafkaConditionReady,
		want: &apis.Condition{
			Type:   KafkaConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark event types",
		s: func() *KafkaSourceStatus {
			s := &KafkaSourceStatus{}
			s.InitializeConditions()
			s.MarkEventTypes()
			return s
		}(),
		condQuery: KafkaConditionReady,
		want: &apis.Condition{
			Type:   KafkaConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark sink and deployed and event types",
		s: func() *KafkaSourceStatus {
			s := &KafkaSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkDeployed(availableDeployment)
			s.MarkEventTypes()
			return s
		}(),
		condQuery: KafkaConditionReady,
		want: &apis.Condition{
			Type:   KafkaConditionReady,
			Status: corev1.ConditionTrue,
		},
	}, {
		name: "mark sink and deployed and event types then no sink",
		s: func() *KafkaSourceStatus {
			s := &KafkaSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkDeployed(availableDeployment)
			s.MarkEventTypes()
			s.MarkNoSink("Testing", "hi%s", "")
			return s
		}(),
		condQuery: KafkaConditionReady,
		want: &apis.Condition{
			Type:    KafkaConditionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "Testing",
			Message: "hi",
		},
	}, {
		name: "mark sink and deployed and event types then deploying",
		s: func() *KafkaSourceStatus {
			s := &KafkaSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkDeployed(availableDeployment)
			s.MarkEventTypes()
			s.MarkDeploying("Testing", "hi%s", "")
			return s
		}(),
		condQuery: KafkaConditionReady,
		want: &apis.Condition{
			Type:    KafkaConditionReady,
			Status:  corev1.ConditionUnknown,
			Reason:  "Testing",
			Message: "hi",
		},
	}, {
		name: "mark sink and deployed and event types then not deployed",
		s: func() *KafkaSourceStatus {
			s := &KafkaSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkDeployed(availableDeployment)
			s.MarkEventTypes()
			s.MarkNotDeployed("Testing", "hi%s", "")
			return s
		}(),
		condQuery: KafkaConditionReady,
		want: &apis.Condition{
			Type:    KafkaConditionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "Testing",
			Message: "hi",
		},
	}, {
		name: "mark sink and deployed and event types then no event types",
		s: func() *KafkaSourceStatus {
			s := &KafkaSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkDeployed(availableDeployment)
			s.MarkEventTypes()
			s.MarkNoEventTypes("Testing", "hi%s", "")
			return s
		}(),
		condQuery: KafkaConditionReady,
		want: &apis.Condition{
			Type:   KafkaConditionReady,
			Status: corev1.ConditionTrue,
		},
	}, {
		name: "mark sink and not deployed then deploying then deployed then event types",
		s: func() *KafkaSourceStatus {
			s := &KafkaSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkNotDeployed("MarkNotDeployed", "%s", "")
			s.MarkDeploying("MarkDeploying", "%s", "")
			s.MarkDeployed(availableDeployment)
			s.MarkEventTypes()
			return s
		}(),
		condQuery: KafkaConditionReady,
		want: &apis.Condition{
			Type:   KafkaConditionReady,
			Status: corev1.ConditionTrue,
		},
	}, {
		name: "mark sink empty and deployed and event types",
		s: func() *KafkaSourceStatus {
			s := &KafkaSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("")
			s.MarkDeployed(availableDeployment)
			s.MarkEventTypes()
			return s
		}(),
		condQuery: KafkaConditionReady,
		want: &apis.Condition{
			Type:    KafkaConditionReady,
			Status:  corev1.ConditionUnknown,
			Reason:  "SinkEmpty",
			Message: "Sink has resolved to empty.",
		},
	}, {
		name: "mark sink empty and deployed and event types then sink",
		s: func() *KafkaSourceStatus {
			s := &KafkaSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("")
			s.MarkDeployed(availableDeployment)
			s.MarkEventTypes()
			s.MarkSink("uri://example")
			return s
		}(),
		condQuery: KafkaConditionReady,
		want: &apis.Condition{
			Type:   KafkaConditionReady,
			Status: corev1.ConditionTrue,
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
