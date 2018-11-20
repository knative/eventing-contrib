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

			k.PropagateContainerSourceStatus(s)
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
			k.PropagateContainerSourceStatus(s)
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
			k.PropagateContainerSourceStatus(s)

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
			k.PropagateContainerSourceStatus(s)

			s.MarkDeployed()
			k.PropagateContainerSourceStatus(s)

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
			k.PropagateContainerSourceStatus(s)

			s.MarkDeployed()
			k.PropagateContainerSourceStatus(s)

			s.MarkNoSink("Testing", "")
			k.PropagateContainerSourceStatus(s)

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
			k.PropagateContainerSourceStatus(s)

			s.MarkDeployed()
			k.PropagateContainerSourceStatus(s)

			s.MarkDeploying("Testing", "")
			k.PropagateContainerSourceStatus(s)

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
			k.PropagateContainerSourceStatus(s)

			s.MarkDeployed()
			k.PropagateContainerSourceStatus(s)

			s.MarkNotDeployed("Testing", "")
			k.PropagateContainerSourceStatus(s)

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
			k.PropagateContainerSourceStatus(s)

			s.MarkNotDeployed("MarkNotDeployed", "")
			k.PropagateContainerSourceStatus(s)

			s.MarkDeploying("MarkDeploying", "")
			k.PropagateContainerSourceStatus(s)

			s.MarkDeployed()
			k.PropagateContainerSourceStatus(s)

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
			k.PropagateContainerSourceStatus(s)

			s.MarkDeployed()
			k.PropagateContainerSourceStatus(s)

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
			k.PropagateContainerSourceStatus(s)

			s.MarkDeployed()
			k.PropagateContainerSourceStatus(s)

			s.MarkSink("uri://example")
			k.PropagateContainerSourceStatus(s)

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
			k.PropagateContainerSourceStatus(s)

			return k
		}(),
		want: duckv1alpha1.Conditions{{
			Type:    KubernetesEventSourceConditionReady,
			Status:  corev1.ConditionUnknown,
			Reason:  "NotSpecified",
			Message: "container source has nil deployed condition",
		}, {
			Type:    KubernetesEventSourceConditionDeployed,
			Status:  corev1.ConditionUnknown,
			Reason:  "NotSpecified",
			Message: "container source has nil deployed condition",
		}, {
			Type:    KubernetesEventSourceConditionSinkProvided,
			Status:  corev1.ConditionUnknown,
			Reason:  "NotSpecified",
			Message: "container source has nil sink provided condition",
		}},
	}, {
		name: "propagate initialized",
		s: func() *KubernetesEventSourceStatus {
			k := &KubernetesEventSourceStatus{}
			k.InitializeConditions()

			s := ContainerSourceStatus{}
			s.InitializeConditions()

			k.PropagateContainerSourceStatus(s)

			return k
		}(),
		want: duckv1alpha1.Conditions{{
			Type:   KubernetesEventSourceConditionReady,
			Status: corev1.ConditionUnknown,
		}, {
			Type:   KubernetesEventSourceConditionDeployed,
			Status: corev1.ConditionUnknown,
		}, {
			Type:   KubernetesEventSourceConditionSinkProvided,
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
			k.PropagateContainerSourceStatus(s)

			return k
		}(),
		want: duckv1alpha1.Conditions{{
			Type:   KubernetesEventSourceConditionReady,
			Status: corev1.ConditionUnknown,
		}, {
			Type:   KubernetesEventSourceConditionDeployed,
			Status: corev1.ConditionTrue,
		}, {
			Type:   KubernetesEventSourceConditionSinkProvided,
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
			k.PropagateContainerSourceStatus(s)

			return k
		}(),
		want: duckv1alpha1.Conditions{{
			Type:   KubernetesEventSourceConditionReady,
			Status: corev1.ConditionUnknown,
		}, {
			Type:   KubernetesEventSourceConditionDeployed,
			Status: corev1.ConditionUnknown,
		}, {
			Type:   KubernetesEventSourceConditionSinkProvided,
			Status: corev1.ConditionTrue,
		}},
	}, {
		name: "propagate mark sink and deployed",
		s: func() *KubernetesEventSourceStatus {
			k := &KubernetesEventSourceStatus{}
			k.InitializeConditions()

			s := ContainerSourceStatus{}
			s.InitializeConditions()

			s.MarkSink("uri://example")
			k.PropagateContainerSourceStatus(s)

			s.MarkDeployed()
			k.PropagateContainerSourceStatus(s)

			return k
		}(),
		want: duckv1alpha1.Conditions{{
			Type:   KubernetesEventSourceConditionReady,
			Status: corev1.ConditionTrue,
		}, {
			Type:   KubernetesEventSourceConditionDeployed,
			Status: corev1.ConditionTrue,
		}, {
			Type:   KubernetesEventSourceConditionSinkProvided,
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
			k.PropagateContainerSourceStatus(s)

			s.MarkDeployed()
			k.PropagateContainerSourceStatus(s)

			s.MarkNoSink("Testing", "hi%s", "")
			k.PropagateContainerSourceStatus(s)

			return k
		}(),
		want: duckv1alpha1.Conditions{{
			Type:    KubernetesEventSourceConditionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "Testing",
			Message: "hi",
		}, {
			Type:   KubernetesEventSourceConditionDeployed,
			Status: corev1.ConditionTrue,
		}, {
			Type:    KubernetesEventSourceConditionSinkProvided,
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
			k.PropagateContainerSourceStatus(s)

			s.MarkDeployed()
			k.PropagateContainerSourceStatus(s)

			s.MarkDeploying("Testing", "hi%s", "")
			k.PropagateContainerSourceStatus(s)

			return k
		}(),
		want: duckv1alpha1.Conditions{{
			Type:    KubernetesEventSourceConditionReady,
			Status:  corev1.ConditionUnknown,
			Reason:  "Testing",
			Message: "hi",
		}, {
			Type:    KubernetesEventSourceConditionDeployed,
			Status:  corev1.ConditionUnknown,
			Reason:  "Testing",
			Message: "hi",
		}, {
			Type:   KubernetesEventSourceConditionSinkProvided,
			Status: corev1.ConditionTrue,
		}},
	}, {
		name: "propagate mark sink and deployed then not deployed",
		s: func() *KubernetesEventSourceStatus {
			k := &KubernetesEventSourceStatus{}
			k.InitializeConditions()

			s := ContainerSourceStatus{}
			s.InitializeConditions()

			s.MarkSink("uri://example")
			k.PropagateContainerSourceStatus(s)

			s.MarkDeployed()
			k.PropagateContainerSourceStatus(s)

			s.MarkNotDeployed("Testing", "hi%s", "")
			k.PropagateContainerSourceStatus(s)

			return k
		}(),
		want: duckv1alpha1.Conditions{{
			Type:    KubernetesEventSourceConditionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "Testing",
			Message: "hi",
		}, {
			Type:    KubernetesEventSourceConditionDeployed,
			Status:  corev1.ConditionFalse,
			Reason:  "Testing",
			Message: "hi",
		}, {
			Type:   KubernetesEventSourceConditionSinkProvided,
			Status: corev1.ConditionTrue,
		}},
	}, {
		name: "propagate mark sink and not deployed then deploying then deployed",
		s: func() *KubernetesEventSourceStatus {
			k := &KubernetesEventSourceStatus{}
			k.InitializeConditions()

			s := ContainerSourceStatus{}
			s.InitializeConditions()

			s.MarkSink("uri://example")
			k.PropagateContainerSourceStatus(s)

			s.MarkNotDeployed("MarkNotDeployed", "%s", "")
			k.PropagateContainerSourceStatus(s)

			s.MarkDeploying("MarkDeploying", "%s", "")
			k.PropagateContainerSourceStatus(s)

			s.MarkDeployed()
			k.PropagateContainerSourceStatus(s)

			return k
		}(),
		want: duckv1alpha1.Conditions{{
			Type:   KubernetesEventSourceConditionReady,
			Status: corev1.ConditionTrue,
		}, {
			Type:   KubernetesEventSourceConditionDeployed,
			Status: corev1.ConditionTrue,
		}, {
			Type:   KubernetesEventSourceConditionSinkProvided,
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
			k.PropagateContainerSourceStatus(s)

			s.MarkDeployed()
			k.PropagateContainerSourceStatus(s)

			return k
		}(),
		want: duckv1alpha1.Conditions{{
			Type:    KubernetesEventSourceConditionReady,
			Status:  corev1.ConditionUnknown,
			Reason:  "SinkEmpty",
			Message: "Sink has resolved to empty.",
		}, {
			Type:   KubernetesEventSourceConditionDeployed,
			Status: corev1.ConditionTrue,
		}, {
			Type:    KubernetesEventSourceConditionSinkProvided,
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
			k.PropagateContainerSourceStatus(s)

			s.MarkDeployed()
			k.PropagateContainerSourceStatus(s)

			s.MarkSink("uri://example")
			k.PropagateContainerSourceStatus(s)

			return k
		}(),
		want: duckv1alpha1.Conditions{{
			Type:   KubernetesEventSourceConditionReady,
			Status: corev1.ConditionTrue,
		}, {
			Type:   KubernetesEventSourceConditionDeployed,
			Status: corev1.ConditionTrue,
		}, {
			Type:   KubernetesEventSourceConditionSinkProvided,
			Status: corev1.ConditionTrue,
		}},
	}}
	// TODO(n3wscott): add a set of tests for PropagateContainerSourceStatus

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
