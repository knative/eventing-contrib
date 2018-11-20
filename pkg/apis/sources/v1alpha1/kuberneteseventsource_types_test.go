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
		name: "propagate empty",
		s: func() *KubernetesEventSourceStatus {
			k := &KubernetesEventSourceStatus{}
			k.InitializeConditions()

			s := ContainerSourceStatus{}

			k.MarkContainerSourceReadyStatus(s)
			return k
		}(),
		want: false,
	}, {
		name: "propagate mark deployed",
		s: func() *KubernetesEventSourceStatus {
			k := &KubernetesEventSourceStatus{}
			k.InitializeConditions()

			s := ContainerSourceStatus{}
			s.InitializeConditions()

			s.MarkDeployed()
			k.MarkContainerSourceReadyStatus(s)
			return k
		}(),
		want: false,
	}, {
		name: "propagate mark sink",
		s: func() *KubernetesEventSourceStatus {
			k := &KubernetesEventSourceStatus{}
			k.InitializeConditions()

			s := ContainerSourceStatus{}
			s.InitializeConditions()

			s.MarkSink("uri://example")
			k.MarkContainerSourceReadyStatus(s)

			return k
		}(),
		want: false,
	}, {
		name: "propagate mark sink and deployed",
		s: func() *KubernetesEventSourceStatus {
			k := &KubernetesEventSourceStatus{}
			k.InitializeConditions()

			s := ContainerSourceStatus{}
			s.InitializeConditions()

			s.MarkSink("uri://example")
			k.MarkContainerSourceReadyStatus(s)

			s.MarkDeployed()
			k.MarkContainerSourceReadyStatus(s)

			return k
		}(),
		want: true,
	}, {
		name: "propagate mark sink and deployed then no sink",
		s: func() *KubernetesEventSourceStatus {
			k := &KubernetesEventSourceStatus{}
			k.InitializeConditions()

			s := ContainerSourceStatus{}
			s.InitializeConditions()

			s.MarkSink("uri://example")
			k.MarkContainerSourceReadyStatus(s)

			s.MarkDeployed()
			k.MarkContainerSourceReadyStatus(s)

			s.MarkNoSink("Testing", "")
			k.MarkContainerSourceReadyStatus(s)

			return k
		}(),
		want: false,
	}, {
		name: "propagate mark sink and deployed then deploying",
		s: func() *KubernetesEventSourceStatus {
			k := &KubernetesEventSourceStatus{}
			k.InitializeConditions()

			s := ContainerSourceStatus{}
			s.InitializeConditions()

			s.MarkSink("uri://example")
			k.MarkContainerSourceReadyStatus(s)

			s.MarkDeployed()
			k.MarkContainerSourceReadyStatus(s)

			s.MarkDeploying("Testing", "")
			k.MarkContainerSourceReadyStatus(s)

			return k
		}(),
		want: false,
	}, {
		name: "propagate mark sink and deployed then not deployed",
		s: func() *KubernetesEventSourceStatus {
			k := &KubernetesEventSourceStatus{}
			k.InitializeConditions()

			s := ContainerSourceStatus{}
			s.InitializeConditions()

			s.MarkSink("uri://example")
			k.MarkContainerSourceReadyStatus(s)

			s.MarkDeployed()
			k.MarkContainerSourceReadyStatus(s)

			s.MarkNotDeployed("Testing", "")
			k.MarkContainerSourceReadyStatus(s)

			return k
		}(),
		want: false,
	}, {
		name: "propagate mark sink and not deployed then deploying then deployed",
		s: func() *KubernetesEventSourceStatus {
			k := &KubernetesEventSourceStatus{}
			k.InitializeConditions()

			s := ContainerSourceStatus{}
			s.InitializeConditions()

			s.MarkSink("uri://example")
			k.MarkContainerSourceReadyStatus(s)

			s.MarkNotDeployed("MarkNotDeployed", "")
			k.MarkContainerSourceReadyStatus(s)

			s.MarkDeploying("MarkDeploying", "")
			k.MarkContainerSourceReadyStatus(s)

			s.MarkDeployed()
			k.MarkContainerSourceReadyStatus(s)

			return k
		}(),
		want: true,
	}, {
		name: "propagate mark sink empty and deployed",
		s: func() *KubernetesEventSourceStatus {
			k := &KubernetesEventSourceStatus{}
			k.InitializeConditions()

			s := ContainerSourceStatus{}
			s.InitializeConditions()

			s.MarkSink("")
			k.MarkContainerSourceReadyStatus(s)

			s.MarkDeployed()
			k.MarkContainerSourceReadyStatus(s)

			return k
		}(),
		want: false,
	}, {
		name: "propagate mark sink empty and deployed then sink",
		s: func() *KubernetesEventSourceStatus {
			k := &KubernetesEventSourceStatus{}
			k.InitializeConditions()

			s := ContainerSourceStatus{}
			s.InitializeConditions()

			s.MarkSink("")
			k.MarkContainerSourceReadyStatus(s)

			s.MarkDeployed()
			k.MarkContainerSourceReadyStatus(s)

			s.MarkSink("uri://example")
			k.MarkContainerSourceReadyStatus(s)

			return k
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

func TestKubernetesEventSourceStatusGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		s         *KubernetesEventSourceStatus
		condQuery duckv1alpha1.ConditionType // Provide a condQuery and [0, 1] want OR > 1 want.
		want      duckv1alpha1.Conditions
	}{{
		name:      "uninitialized",
		s:         &KubernetesEventSourceStatus{},
		condQuery: KubernetesEventSourceConditionReady,
		want:      nil,
	}, {
		name: "initialized",
		s: func() *KubernetesEventSourceStatus {
			k := &KubernetesEventSourceStatus{}
			k.InitializeConditions()

			s := &KubernetesEventSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		want: duckv1alpha1.Conditions{{
			Type:   KubernetesEventSourceConditionReady,
			Status: corev1.ConditionUnknown,
		}},
	}, {
		name: "propagate uninitialized",
		s: func() *KubernetesEventSourceStatus {
			k := &KubernetesEventSourceStatus{}
			k.InitializeConditions()

			s := ContainerSourceStatus{}
			k.MarkContainerSourceReadyStatus(s)

			return k
		}(),
		want: duckv1alpha1.Conditions{{
			Type:    KubernetesEventSourceConditionReady,
			Status:  corev1.ConditionUnknown,
			Reason:  "SinkProvidedDeploy",
			Message: "SinkProvided status is nil; Deploy status is nil",
		}, {
			Type:    KubernetesEventSourceContainerSourceReady,
			Status:  corev1.ConditionUnknown,
			Reason:  "SinkProvidedDeploy",
			Message: "SinkProvided status is nil; Deploy status is nil",
		}},
	}, {
		name: "propagate initialized",
		s: func() *KubernetesEventSourceStatus {
			k := &KubernetesEventSourceStatus{}
			k.InitializeConditions()

			s := ContainerSourceStatus{}
			s.InitializeConditions()

			k.MarkContainerSourceReadyStatus(s)

			return k
		}(),
		want: duckv1alpha1.Conditions{{
			Type:   KubernetesEventSourceConditionReady,
			Status: corev1.ConditionUnknown,
		}, {
			Type:   KubernetesEventSourceContainerSourceReady,
			Status: corev1.ConditionUnknown,
		}},
	}, {
		name: "propagate mark deployed",
		s: func() *KubernetesEventSourceStatus {
			k := &KubernetesEventSourceStatus{}
			k.InitializeConditions()

			s := ContainerSourceStatus{}
			s.InitializeConditions()

			s.MarkDeployed()
			k.MarkContainerSourceReadyStatus(s)

			return k
		}(),
		want: duckv1alpha1.Conditions{{
			Type:   KubernetesEventSourceConditionReady,
			Status: corev1.ConditionUnknown,
		}, {
			Type:   KubernetesEventSourceContainerSourceReady,
			Status: corev1.ConditionUnknown,
		}},
	}, {
		name: "propagate mark sink",
		s: func() *KubernetesEventSourceStatus {
			k := &KubernetesEventSourceStatus{}
			k.InitializeConditions()

			s := ContainerSourceStatus{}
			s.InitializeConditions()

			s.MarkSink("uri://example")
			k.MarkContainerSourceReadyStatus(s)

			return k
		}(),
		want: duckv1alpha1.Conditions{{
			Type:   KubernetesEventSourceConditionReady,
			Status: corev1.ConditionUnknown,
		}, {
			Type:   KubernetesEventSourceContainerSourceReady,
			Status: corev1.ConditionUnknown,
		}},
	}, {
		name: "propagate mark sink and deployed",
		s: func() *KubernetesEventSourceStatus {
			k := &KubernetesEventSourceStatus{}
			k.InitializeConditions()

			s := ContainerSourceStatus{}
			s.InitializeConditions()

			s.MarkSink("uri://example")
			k.MarkContainerSourceReadyStatus(s)

			s.MarkDeployed()
			k.MarkContainerSourceReadyStatus(s)

			return k
		}(),
		want: duckv1alpha1.Conditions{{
			Type:   KubernetesEventSourceConditionReady,
			Status: corev1.ConditionTrue,
		}, {
			Type:   KubernetesEventSourceContainerSourceReady,
			Status: corev1.ConditionTrue,
		}},
	}, {
		name: "propagate mark sink and deployed then no sink",
		s: func() *KubernetesEventSourceStatus {
			k := &KubernetesEventSourceStatus{}
			k.InitializeConditions()

			s := ContainerSourceStatus{}
			s.InitializeConditions()

			s.MarkSink("uri://example")
			k.MarkContainerSourceReadyStatus(s)

			s.MarkDeployed()
			k.MarkContainerSourceReadyStatus(s)

			s.MarkNoSink("Testing", "hi%s", "")
			k.MarkContainerSourceReadyStatus(s)

			return k
		}(),
		want: duckv1alpha1.Conditions{{
			Type:    KubernetesEventSourceConditionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "Testing",
			Message: "hi",
		}, {
			Type:    KubernetesEventSourceContainerSourceReady,
			Status:  corev1.ConditionFalse,
			Reason:  "Testing",
			Message: "hi",
		}},
	}, {
		name: "propagate mark sink and deployed then deploying",
		s: func() *KubernetesEventSourceStatus {
			k := &KubernetesEventSourceStatus{}
			k.InitializeConditions()

			s := ContainerSourceStatus{}
			s.InitializeConditions()

			s.MarkSink("uri://example")
			k.MarkContainerSourceReadyStatus(s)

			s.MarkDeployed()
			k.MarkContainerSourceReadyStatus(s)

			s.MarkDeploying("Testing", "hi%s", "")
			k.MarkContainerSourceReadyStatus(s)

			return k
		}(),
		want: duckv1alpha1.Conditions{{
			Type:    KubernetesEventSourceConditionReady,
			Status:  corev1.ConditionUnknown,
			Reason:  "Testing",
			Message: "hi",
		}, {
			Type:    KubernetesEventSourceContainerSourceReady,
			Status:  corev1.ConditionUnknown,
			Reason:  "Testing",
			Message: "hi",
		}},
	}, {
		name: "propagate mark sink and deployed then not deployed",
		s: func() *KubernetesEventSourceStatus {
			k := &KubernetesEventSourceStatus{}
			k.InitializeConditions()

			s := ContainerSourceStatus{}
			s.InitializeConditions()

			s.MarkSink("uri://example")
			k.MarkContainerSourceReadyStatus(s)

			s.MarkDeployed()
			k.MarkContainerSourceReadyStatus(s)

			s.MarkNotDeployed("Testing", "hi%s", "")
			k.MarkContainerSourceReadyStatus(s)

			return k
		}(),
		want: duckv1alpha1.Conditions{{
			Type:    KubernetesEventSourceConditionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "Testing",
			Message: "hi",
		}, {
			Type:    KubernetesEventSourceContainerSourceReady,
			Status:  corev1.ConditionFalse,
			Reason:  "Testing",
			Message: "hi",
		}},
	}, {
		name: "propagate mark sink and not deployed then deploying then deployed",
		s: func() *KubernetesEventSourceStatus {
			k := &KubernetesEventSourceStatus{}
			k.InitializeConditions()

			s := ContainerSourceStatus{}
			s.InitializeConditions()

			s.MarkSink("uri://example")
			k.MarkContainerSourceReadyStatus(s)

			s.MarkNotDeployed("MarkNotDeployed", "%s", "")
			k.MarkContainerSourceReadyStatus(s)

			s.MarkDeploying("MarkDeploying", "%s", "")
			k.MarkContainerSourceReadyStatus(s)

			s.MarkDeployed()
			k.MarkContainerSourceReadyStatus(s)

			return k
		}(),
		want: duckv1alpha1.Conditions{{
			Type:   KubernetesEventSourceConditionReady,
			Status: corev1.ConditionTrue,
		}, {
			Type:   KubernetesEventSourceContainerSourceReady,
			Status: corev1.ConditionTrue,
		}},
	}, {
		name: "propagate mark sink empty and deployed",
		s: func() *KubernetesEventSourceStatus {
			k := &KubernetesEventSourceStatus{}
			k.InitializeConditions()

			s := ContainerSourceStatus{}
			s.InitializeConditions()

			s.MarkSink("")
			k.MarkContainerSourceReadyStatus(s)

			s.MarkDeployed()
			k.MarkContainerSourceReadyStatus(s)

			return k
		}(),
		want: duckv1alpha1.Conditions{{
			Type:    KubernetesEventSourceConditionReady,
			Status:  corev1.ConditionUnknown,
			Reason:  "SinkEmpty",
			Message: "Sink has resolved to empty.",
		}, {
			Type:    KubernetesEventSourceContainerSourceReady,
			Status:  corev1.ConditionUnknown,
			Reason:  "SinkEmpty",
			Message: "Sink has resolved to empty.",
		}},
	}, {
		name: "propagate mark sink empty and deployed then sink",
		s: func() *KubernetesEventSourceStatus {
			k := &KubernetesEventSourceStatus{}
			k.InitializeConditions()

			s := ContainerSourceStatus{}
			s.InitializeConditions()

			s.MarkSink("")
			k.MarkContainerSourceReadyStatus(s)

			s.MarkDeployed()
			k.MarkContainerSourceReadyStatus(s)

			s.MarkSink("uri://example")
			k.MarkContainerSourceReadyStatus(s)

			return k
		}(),
		want: duckv1alpha1.Conditions{{
			Type:   KubernetesEventSourceConditionReady,
			Status: corev1.ConditionTrue,
		}, {
			Type:   KubernetesEventSourceContainerSourceReady,
			Status: corev1.ConditionTrue,
		}},
	}}
	// TODO(n3wscott): add a set of tests for MarkContainerSourceReadyStatus

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ignoreTime := cmpopts.IgnoreFields(duckv1alpha1.Condition{}, "LastTransitionTime")

			if test.condQuery != "" {
				got := test.s.GetCondition(test.condQuery)
				var want *duckv1alpha1.Condition
				if len(test.want) == 1 {
					want = &test.want[0]
				} else if len(test.want) > 1 {
					t.Fatalf("error: test provides a query for conditions and more than one expected condition. Fix the test.")
				}
				if diff := cmp.Diff(want, got, ignoreTime); diff != "" {
					t.Errorf("unexpected condition (-want, +got) = %v", diff)
				}
				return
			}

			// no condition query provided, search for all in want.

			for _, v := range test.want {
				got := test.s.GetCondition(v.Type)
				want := &v
				if diff := cmp.Diff(want, got, ignoreTime); diff != "" {
					t.Errorf("unexpected condition (-want, +got) = %v", diff)
				}
			}
		})
	}
}
