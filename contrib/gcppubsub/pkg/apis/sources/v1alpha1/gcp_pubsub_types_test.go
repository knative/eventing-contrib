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

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func TestGcpPubSubSourceStatusIsReady(t *testing.T) {
	tests := []struct {
		name string
		s    *GcpPubSubSourceStatus
		want bool
	}{{
		name: "uninitialized",
		s:    &GcpPubSubSourceStatus{},
		want: false,
	}, {
		name: "initialized",
		s: func() *GcpPubSubSourceStatus {
			s := &GcpPubSubSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		want: false,
	}, {
		name: "mark deployed",
		s: func() *GcpPubSubSourceStatus {
			s := &GcpPubSubSourceStatus{}
			s.InitializeConditions()
			s.MarkDeployed()
			return s
		}(),
		want: false,
	}, {
		name: "mark sink",
		s: func() *GcpPubSubSourceStatus {
			s := &GcpPubSubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			return s
		}(),
		want: false,
	}, {
		name: "mark subscribed",
		s: func() *GcpPubSubSourceStatus {
			s := &GcpPubSubSourceStatus{}
			s.InitializeConditions()
			s.MarkSubscribed()
			return s
		}(),
		want: false,
	}, {
		name: "mark event types",
		s: func() *GcpPubSubSourceStatus {
			s := &GcpPubSubSourceStatus{}
			s.InitializeConditions()
			s.MarkEventTypes()
			return s
		}(),
		want: false,
	}, {
		name: "mark sink and deployed",
		s: func() *GcpPubSubSourceStatus {
			s := &GcpPubSubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkDeployed()
			return s
		}(),
		want: false,
	}, {
		name: "mark sink and deployed and subscribed and event types",
		s: func() *GcpPubSubSourceStatus {
			s := &GcpPubSubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkDeployed()
			s.MarkSubscribed()
			s.MarkEventTypes()
			return s
		}(),
		want: true,
	}, {
		name: "mark sink and deployed and subscribed and event types, then no sink",
		s: func() *GcpPubSubSourceStatus {
			s := &GcpPubSubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkDeployed()
			s.MarkSubscribed()
			s.MarkEventTypes()
			s.MarkNoSink("Testing", "")
			return s
		}(),
		want: false,
	}, {
		name: "mark sink and deployed and subscribed and event types then deploying",
		s: func() *GcpPubSubSourceStatus {
			s := &GcpPubSubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkDeployed()
			s.MarkSubscribed()
			s.MarkEventTypes()
			s.MarkDeploying("Testing", "")
			return s
		}(),
		want: false,
	}, {
		name: "mark sink and deployed and subscribed and event types then not deployed",
		s: func() *GcpPubSubSourceStatus {
			s := &GcpPubSubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkDeployed()
			s.MarkSubscribed()
			s.MarkEventTypes()
			s.MarkNotDeployed("Testing", "")
			return s
		}(),
		want: false,
	}, {
		name: "mark sink and deployed and subscribed and event types then no event types",
		s: func() *GcpPubSubSourceStatus {
			s := &GcpPubSubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkDeployed()
			s.MarkSubscribed()
			s.MarkEventTypes()
			s.MarkNoEventTypes("Testing", "")
			return s
		}(),
		want: true,
	}, {
		name: "mark sink and subscribed and not deployed then deploying then deployed then event types",
		s: func() *GcpPubSubSourceStatus {
			s := &GcpPubSubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkSubscribed()
			s.MarkNotDeployed("MarkNotDeployed", "")
			s.MarkDeploying("MarkDeploying", "")
			s.MarkDeployed()
			s.MarkEventTypes()
			return s
		}(),
		want: true,
	}, {
		name: "mark sink empty and deployed and subscribed and event types",
		s: func() *GcpPubSubSourceStatus {
			s := &GcpPubSubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("")
			s.MarkDeployed()
			s.MarkSubscribed()
			s.MarkEventTypes()
			return s
		}(),
		want: false,
	}, {
		name: "mark sink empty and deployed and subscribed and event types then sink",
		s: func() *GcpPubSubSourceStatus {
			s := &GcpPubSubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("")
			s.MarkDeployed()
			s.MarkSubscribed()
			s.MarkEventTypes()
			s.MarkSink("uri://example")
			return s
		}(),
		want: true,
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

func TestGcpPubSubSourceStatusGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		s         *GcpPubSubSourceStatus
		condQuery duckv1alpha1.ConditionType
		want      *duckv1alpha1.Condition
	}{{
		name:      "uninitialized",
		s:         &GcpPubSubSourceStatus{},
		condQuery: GcpPubSubConditionReady,
		want:      nil,
	}, {
		name: "initialized",
		s: func() *GcpPubSubSourceStatus {
			s := &GcpPubSubSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		condQuery: GcpPubSubConditionReady,
		want: &duckv1alpha1.Condition{
			Type:   GcpPubSubConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark deployed",
		s: func() *GcpPubSubSourceStatus {
			s := &GcpPubSubSourceStatus{}
			s.InitializeConditions()
			s.MarkDeployed()
			return s
		}(),
		condQuery: GcpPubSubConditionReady,
		want: &duckv1alpha1.Condition{
			Type:   GcpPubSubConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark sink",
		s: func() *GcpPubSubSourceStatus {
			s := &GcpPubSubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			return s
		}(),
		condQuery: GcpPubSubConditionReady,
		want: &duckv1alpha1.Condition{
			Type:   GcpPubSubConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark subscribed",
		s: func() *GcpPubSubSourceStatus {
			s := &GcpPubSubSourceStatus{}
			s.InitializeConditions()
			s.MarkSubscribed()
			return s
		}(),
		condQuery: GcpPubSubConditionReady,
		want: &duckv1alpha1.Condition{
			Type:   GcpPubSubConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark event types",
		s: func() *GcpPubSubSourceStatus {
			s := &GcpPubSubSourceStatus{}
			s.InitializeConditions()
			s.MarkEventTypes()
			return s
		}(),
		condQuery: GcpPubSubConditionReady,
		want: &duckv1alpha1.Condition{
			Type:   GcpPubSubConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark sink and deployed",
		s: func() *GcpPubSubSourceStatus {
			s := &GcpPubSubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkDeployed()
			return s
		}(),
		condQuery: GcpPubSubConditionReady,
		want: &duckv1alpha1.Condition{
			Type:   GcpPubSubConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark sink and deployed and subscribed and event types",
		s: func() *GcpPubSubSourceStatus {
			s := &GcpPubSubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkDeployed()
			s.MarkSubscribed()
			s.MarkEventTypes()
			return s
		}(),
		condQuery: GcpPubSubConditionReady,
		want: &duckv1alpha1.Condition{
			Type:   GcpPubSubConditionReady,
			Status: corev1.ConditionTrue,
		},
	}, {
		name: "mark sink and deployed and subscribed and event types then no sink",
		s: func() *GcpPubSubSourceStatus {
			s := &GcpPubSubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkDeployed()
			s.MarkSubscribed()
			s.MarkEventTypes()
			s.MarkNoSink("Testing", "hi%s", "")
			return s
		}(),
		condQuery: GcpPubSubConditionReady,
		want: &duckv1alpha1.Condition{
			Type:    GcpPubSubConditionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "Testing",
			Message: "hi",
		},
	}, {
		name: "mark sink and deployed and subscribed and event types then deploying",
		s: func() *GcpPubSubSourceStatus {
			s := &GcpPubSubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkDeployed()
			s.MarkSubscribed()
			s.MarkEventTypes()
			s.MarkDeploying("Testing", "hi%s", "")
			return s
		}(),
		condQuery: GcpPubSubConditionReady,
		want: &duckv1alpha1.Condition{
			Type:    GcpPubSubConditionReady,
			Status:  corev1.ConditionUnknown,
			Reason:  "Testing",
			Message: "hi",
		},
	}, {
		name: "mark sink and deployed and subscribed and event types then not deployed",
		s: func() *GcpPubSubSourceStatus {
			s := &GcpPubSubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkDeployed()
			s.MarkSubscribed()
			s.MarkEventTypes()
			s.MarkNotDeployed("Testing", "hi%s", "")
			return s
		}(),
		condQuery: GcpPubSubConditionReady,
		want: &duckv1alpha1.Condition{
			Type:    GcpPubSubConditionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "Testing",
			Message: "hi",
		},
	}, {
		name: "mark sink and deployed and subscribed and event types then no event types",
		s: func() *GcpPubSubSourceStatus {
			s := &GcpPubSubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkDeployed()
			s.MarkSubscribed()
			s.MarkEventTypes()
			s.MarkNoEventTypes("Testing", "hi%s", "")
			return s
		}(),
		condQuery: GcpPubSubConditionReady,
		want: &duckv1alpha1.Condition{
			Type:   GcpPubSubConditionReady,
			Status: corev1.ConditionTrue,
		},
	}, {
		name: "mark sink and subscribed and not deployed then deploying then deployed then event types",
		s: func() *GcpPubSubSourceStatus {
			s := &GcpPubSubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkSubscribed()
			s.MarkNotDeployed("MarkNotDeployed", "%s", "")
			s.MarkDeploying("MarkDeploying", "%s", "")
			s.MarkDeployed()
			s.MarkEventTypes()
			return s
		}(),
		condQuery: GcpPubSubConditionReady,
		want: &duckv1alpha1.Condition{
			Type:   GcpPubSubConditionReady,
			Status: corev1.ConditionTrue,
		},
	}, {
		name: "mark sink empty and deployed and subscribed and event types",
		s: func() *GcpPubSubSourceStatus {
			s := &GcpPubSubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("")
			s.MarkDeployed()
			s.MarkSubscribed()
			s.MarkEventTypes()
			return s
		}(),
		condQuery: GcpPubSubConditionReady,
		want: &duckv1alpha1.Condition{
			Type:    GcpPubSubConditionReady,
			Status:  corev1.ConditionUnknown,
			Reason:  "SinkEmpty",
			Message: "Sink has resolved to empty.",
		},
	}, {
		name: "mark sink empty and deployed and subscribed and event types then sink",
		s: func() *GcpPubSubSourceStatus {
			s := &GcpPubSubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("")
			s.MarkDeployed()
			s.MarkSubscribed()
			s.MarkSink("uri://example")
			s.MarkEventTypes()
			return s
		}(),
		condQuery: GcpPubSubConditionReady,
		want: &duckv1alpha1.Condition{
			Type:   GcpPubSubConditionReady,
			Status: corev1.ConditionTrue,
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
