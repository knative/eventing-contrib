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
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
)

func TestBitBucketSourceStatusIsReady(t *testing.T) {
	tests := []struct {
		name string
		s    *BitBucketSourceStatus
		want bool
	}{{
		name: "uninitialized",
		s:    &BitBucketSourceStatus{},
		want: false,
	}, {
		name: "initialized",
		s: func() *BitBucketSourceStatus {
			s := &BitBucketSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		want: false,
	}, {
		name: "mark sink",
		s: func() *BitBucketSourceStatus {
			s := &BitBucketSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			return s
		}(),
		want: false,
	}, {
		name: "mark secrets",
		s: func() *BitBucketSourceStatus {
			s := &BitBucketSourceStatus{}
			s.InitializeConditions()
			s.MarkSecrets()
			return s
		}(),
		want: false,
	}, {
		name: "mark sink and secrets",
		s: func() *BitBucketSourceStatus {
			s := &BitBucketSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkSecrets()
			return s
		}(),
		want: false,
	}, {
		name: "mark sink, secrets, and service",
		s: func() *BitBucketSourceStatus {
			s := &BitBucketSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkSecrets()
			s.MarkService()
			return s
		}(),
		want: false,
	}, {
		name: "mark sink, secrets, service, and webhook",
		s: func() *BitBucketSourceStatus {
			s := &BitBucketSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkSecrets()
			s.MarkService()
			s.MarkWebHook("uri://webhook")
			return s
		}(),
		want: true,
	}, {
		name: "mark sink, secrets, service, and webhook, then no service",
		s: func() *BitBucketSourceStatus {
			s := &BitBucketSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkSecrets()
			s.MarkService()
			s.MarkWebHook("uri://webhook")
			s.MarkNoService("Testing", "")
			return s
		}(),
		want: false,
	}, {
		name: "mark sink, secrets, service, and empty webhook",
		s: func() *BitBucketSourceStatus {
			s := &BitBucketSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkSecrets()
			s.MarkService()
			s.MarkWebHook("")
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

func TestBitBucketSourceStatusGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		s         *BitBucketSourceStatus
		condQuery duckv1alpha1.ConditionType
		want      *duckv1alpha1.Condition
	}{{
		name:      "uninitialized",
		s:         &BitBucketSourceStatus{},
		condQuery: BitBucketSourceConditionReady,
		want:      nil,
	}, {
		name: "initialized",
		s: func() *BitBucketSourceStatus {
			s := &BitBucketSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		condQuery: BitBucketSourceConditionReady,
		want: &duckv1alpha1.Condition{
			Type:   BitBucketSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark sink",
		s: func() *BitBucketSourceStatus {
			s := &BitBucketSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			return s
		}(),
		condQuery: BitBucketSourceConditionReady,
		want: &duckv1alpha1.Condition{
			Type:   BitBucketSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark sink and secrets",
		s: func() *BitBucketSourceStatus {
			s := &BitBucketSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkSecrets()
			return s
		}(),
		condQuery: BitBucketSourceConditionReady,
		want: &duckv1alpha1.Condition{
			Type:   BitBucketSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark sink, secrets, and service",
		s: func() *BitBucketSourceStatus {
			s := &BitBucketSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkSecrets()
			s.MarkService()
			return s
		}(),
		condQuery: BitBucketSourceConditionReady,
		want: &duckv1alpha1.Condition{
			Type:   BitBucketSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark sink, secrets, service, and webhook",
		s: func() *BitBucketSourceStatus {
			s := &BitBucketSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkSecrets()
			s.MarkService()
			s.MarkWebHook("uri://webhook")
			return s
		}(),
		condQuery: BitBucketSourceConditionReady,
		want: &duckv1alpha1.Condition{
			Type:   BitBucketSourceConditionReady,
			Status: corev1.ConditionTrue,
		},
	}, {
		name: "mark sink, secrets, service, and empty webhook",
		s: func() *BitBucketSourceStatus {
			s := &BitBucketSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkSecrets()
			s.MarkService()
			s.MarkWebHook("")
			return s
		}(),
		condQuery: BitBucketSourceConditionReady,
		want: &duckv1alpha1.Condition{
			Type:    BitBucketSourceConditionReady,
			Status:  corev1.ConditionUnknown,
			Reason:  "WebHookUUIDEmpty",
			Message: "WebHookUUID is empty.",
		},
	}, {
		name: "mark sink, secrets, service, webhook, and the no sink",
		s: func() *BitBucketSourceStatus {
			s := &BitBucketSourceStatus{}
			s.InitializeConditions()
			s.MarkSink("uri://example")
			s.MarkSecrets()
			s.MarkService()
			s.MarkWebHook("uri://webhook")
			s.MarkNoSink("Testing", "%s", "")
			return s
		}(),
		condQuery: BitBucketSourceConditionReady,
		want: &duckv1alpha1.Condition{
			Type:    BitBucketSourceConditionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "Testing",
			Message: "",
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
