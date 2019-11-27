package v1alpha1

import (
	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"testing"
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
		Type: RabbitmqConditionReady,
		Status: corev1.ConditionTrue,
	}
)

var _ = duck.VerifyType(&RabbitmqSource{}, &duckv1.Conditions{})

func TestRabbitmqSourceStatusGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		s         *RabbitmqSourceStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name:      "uninitialized",
		s:         &RabbitmqSourceStatus{},
		condQuery: RabbitmqConditionReady,
		want:      nil,
	}, {
		name: "initialized",
		s: func() *RabbitmqSourceStatus {
			s := &RabbitmqSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		condQuery: RabbitmqConditionReady,
		want: &apis.Condition{
			Type:   RabbitmqConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark deployed",
		s: func() *RabbitmqSourceStatus {
			s := &RabbitmqSourceStatus{}
			s.InitializeConditions()
			s.MarkDeployed(availableDeployment)
			return s
		}(),
		condQuery: RabbitmqConditionReady,
		want: &apis.Condition{
			Type:   RabbitmqConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark sink",
		s: func() *RabbitmqSourceStatus {
			s := &RabbitmqSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(apis.HTTP("uri://example"))
			return s
		}(),
		condQuery: RabbitmqConditionReady,
		want: &apis.Condition{
			Type:   RabbitmqConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark event types",
		s: func() *RabbitmqSourceStatus {
			s := &RabbitmqSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		condQuery: RabbitmqConditionReady,
		want: &apis.Condition{
			Type:   RabbitmqConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark sink and deployed and event types",
		s: func() *RabbitmqSourceStatus {
			s := &RabbitmqSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(apis.HTTP("uri://example"))
			s.MarkDeployed(availableDeployment)
			return s
		}(),
		condQuery: RabbitmqConditionReady,
		want: &apis.Condition{
			Type:   RabbitmqConditionReady,
			Status: corev1.ConditionTrue,
		},
	}, {
		name: "mark sink and deployed and event types then no sink",
		s: func() *RabbitmqSourceStatus {
			s := &RabbitmqSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(apis.HTTP("uri://example"))
			s.MarkDeployed(availableDeployment)
			s.MarkNoSink("Testing", "hi%s", "")
			return s
		}(),
		condQuery: RabbitmqConditionReady,
		want: &apis.Condition{
			Type:    RabbitmqConditionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "Testing",
			Message: "hi",
		},
	}, {
		name: "mark sink and deployed and event types then deploying",
		s: func() *RabbitmqSourceStatus {
			s := &RabbitmqSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(apis.HTTP("uri://example"))
			s.MarkDeployed(availableDeployment)
			s.MarkDeploying("Testing", "hi%s", "")
			return s
		}(),
		condQuery: RabbitmqConditionReady,
		want: &apis.Condition{
			Type:    RabbitmqConditionReady,
			Status:  corev1.ConditionUnknown,
			Reason:  "Testing",
			Message: "hi",
		},
	}, {
		name: "mark sink and deployed and event types then not deployed",
		s: func() *RabbitmqSourceStatus {
			s := &RabbitmqSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(apis.HTTP("uri://example"))
			s.MarkDeployed(availableDeployment)
			s.MarkNotDeployed("Testing", "hi%s", "")
			return s
		}(),
		condQuery: RabbitmqConditionReady,
		want: &apis.Condition{
			Type:    RabbitmqConditionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "Testing",
			Message: "hi",
		},
	}, {
		name: "mark sink and deployed",
		s: func() *RabbitmqSourceStatus {
			s := &RabbitmqSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(apis.HTTP("uri://example"))
			s.MarkDeployed(availableDeployment)
			return s
		}(),
		condQuery: RabbitmqConditionReady,
		want: &apis.Condition{
			Type:   RabbitmqConditionReady,
			Status: corev1.ConditionTrue,
		},
	}, {
		name: "mark sink and not deployed then deploying then deployed",
		s: func() *RabbitmqSourceStatus {
			s := &RabbitmqSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(apis.HTTP("uri://example"))
			s.MarkNotDeployed("MarkNotDeployed", "%s", "")
			s.MarkDeploying("MarkDeploying", "%s", "")
			s.MarkDeployed(availableDeployment)
			return s
		}(),
		condQuery: RabbitmqConditionReady,
		want: &apis.Condition{
			Type:   RabbitmqConditionReady,
			Status: corev1.ConditionTrue,
		},
	}, {
		name: "mark sink empty and deployed and event types",
		s: func() *RabbitmqSourceStatus {
			s := &RabbitmqSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(nil)
			s.MarkDeployed(availableDeployment)
			return s
		}(),
		condQuery: RabbitmqConditionReady,
		want: &apis.Condition{
			Type:    RabbitmqConditionReady,
			Status:  corev1.ConditionUnknown,
			Reason:  "SinkEmpty",
			Message: "Sink has resolved to empty.",
		},
	}, {
		name: "mark sink empty and deployed then sink",
		s: func() *RabbitmqSourceStatus {
			s := &RabbitmqSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(nil)
			s.MarkDeployed(availableDeployment)
			s.MarkSink(apis.HTTP("uri://example"))
			return s
		}(),
		condQuery: RabbitmqConditionReady,
		want: &apis.Condition{
			Type:   RabbitmqConditionReady,
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