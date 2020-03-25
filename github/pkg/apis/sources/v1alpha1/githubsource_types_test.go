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
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var _ = duck.VerifyType(&GitHubSource{}, &duckv1.Conditions{})

func TestGitHubSourceStatusIsReady(t *testing.T) {
	tests := []struct {
		name string
		s    *GitHubSourceStatus
		want bool
	}{{
		name: "uninitialized",
		s:    &GitHubSourceStatus{},
		want: false,
	}, {
		name: "initialized",
		s: func() *GitHubSourceStatus {
			s := &GitHubSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		want: false,
	}, {
		name: "mark sink",
		s: func() *GitHubSourceStatus {
			s := &GitHubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(apis.HTTP("example"))
			return s
		}(),
		want: false,
	}, {
		name: "mark secrets",
		s: func() *GitHubSourceStatus {
			s := &GitHubSourceStatus{}
			s.InitializeConditions()
			s.MarkSecrets()
			return s
		}(),
		want: false,
	}, {
		name: "mark webhook",
		s: func() *GitHubSourceStatus {
			s := &GitHubSourceStatus{}
			s.InitializeConditions()
			s.MarkSecrets()
			return s
		}(),
		want: false,
	}, {
		name: "mark sink, secrets, webhook",
		s: func() *GitHubSourceStatus {
			s := &GitHubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(apis.HTTP("example"))
			s.MarkSecrets()
			s.MarkWebhookConfigured()
			return s
		}(),
		want: true,
	}, {
		name: "mark sink, secrets, webhook, then no sink",
		s: func() *GitHubSourceStatus {
			s := &GitHubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(apis.HTTP("example"))
			s.MarkSecrets()
			s.MarkWebhookConfigured()
			s.MarkNoSink("Testing", "")
			return s
		}(),
		want: false,
	}, {
		name: "mark sink, secrets, webhook, then no secrets",
		s: func() *GitHubSourceStatus {
			s := &GitHubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(apis.HTTP("example"))
			s.MarkSecrets()
			s.MarkWebhookConfigured()
			s.MarkNoSecrets("Testing", "")
			return s
		}(),
		want: false,
	}, {
		name: "mark sink, secrets, webhook, then no webhook",
		s: func() *GitHubSourceStatus {
			s := &GitHubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(apis.HTTP("example"))
			s.MarkSecrets()
			s.MarkWebhookConfigured()
			s.MarkWebhookNotConfigured("Testing", "")
			return s
		}(),
		want: false,
	}, {
		name: "mark sink nil, secrets, webhook",
		s: func() *GitHubSourceStatus {
			s := &GitHubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(nil)
			s.MarkSecrets()
			s.MarkWebhookConfigured()
			return s
		}(),
		want: false,
	}, {
		name: "mark sink nil, secrets, webhook, then sink",
		s: func() *GitHubSourceStatus {
			s := &GitHubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(nil)
			s.MarkSecrets()
			s.MarkWebhookConfigured()
			s.MarkSink(apis.HTTP("example"))
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

func TestGitHubSourceStatusGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		s         *GitHubSourceStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name:      "uninitialized",
		s:         &GitHubSourceStatus{},
		condQuery: GitHubSourceConditionReady,
		want:      nil,
	}, {
		name: "initialized",
		s: func() *GitHubSourceStatus {
			s := &GitHubSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		condQuery: GitHubSourceConditionReady,
		want: &apis.Condition{
			Type:   GitHubSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark sink",
		s: func() *GitHubSourceStatus {
			s := &GitHubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(apis.HTTP("example"))
			return s
		}(),
		condQuery: GitHubSourceConditionReady,
		want: &apis.Condition{
			Type:   GitHubSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark secrets",
		s: func() *GitHubSourceStatus {
			s := &GitHubSourceStatus{}
			s.InitializeConditions()
			s.MarkSecrets()
			return s
		}(),
		condQuery: GitHubSourceConditionReady,
		want: &apis.Condition{
			Type:   GitHubSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark webhook",
		s: func() *GitHubSourceStatus {
			s := &GitHubSourceStatus{}
			s.InitializeConditions()
			s.MarkWebhookConfigured()
			return s
		}(),
		condQuery: GitHubSourceConditionReady,
		want: &apis.Condition{
			Type:   GitHubSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark sink, secrets, webhook",
		s: func() *GitHubSourceStatus {
			s := &GitHubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(apis.HTTP("example"))
			s.MarkSecrets()
			s.MarkWebhookConfigured()
			return s
		}(),
		condQuery: GitHubSourceConditionReady,
		want: &apis.Condition{
			Type:   GitHubSourceConditionReady,
			Status: corev1.ConditionTrue,
		},
	}, {
		name: "mark sink, secrets, webhook, then no sink",
		s: func() *GitHubSourceStatus {
			s := &GitHubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(apis.HTTP("example"))
			s.MarkSecrets()
			s.MarkWebhookConfigured()
			s.MarkNoSink("Testing", "hi%s", "")
			return s
		}(),
		condQuery: GitHubSourceConditionReady,
		want: &apis.Condition{
			Type:    GitHubSourceConditionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "Testing",
			Message: "hi",
		},
	}, {
		name: "mark sink, secrets, webhook, then no secrets",
		s: func() *GitHubSourceStatus {
			s := &GitHubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(apis.HTTP("example"))
			s.MarkSecrets()
			s.MarkWebhookConfigured()
			s.MarkNoSecrets("Testing", "hi%s", "")
			return s
		}(),
		condQuery: GitHubSourceConditionReady,
		want: &apis.Condition{
			Type:    GitHubSourceConditionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "Testing",
			Message: "hi",
		},
	}, {
		name: "mark sink, secrets, webhook, then no webhook",
		s: func() *GitHubSourceStatus {
			s := &GitHubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(apis.HTTP("example"))
			s.MarkSecrets()
			s.MarkWebhookConfigured()
			s.MarkWebhookNotConfigured("Testing", "hi%s", "")
			return s
		}(),
		condQuery: GitHubSourceConditionReady,
		want: &apis.Condition{
			Type:    GitHubSourceConditionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "Testing",
			Message: "hi",
		},
	}, {
		name: "mark sink nil, secrets, webhook",
		s: func() *GitHubSourceStatus {
			s := &GitHubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(nil)
			s.MarkSecrets()
			s.MarkWebhookConfigured()
			return s
		}(),
		condQuery: GitHubSourceConditionReady,
		want: &apis.Condition{
			Type:    GitHubSourceConditionReady,
			Status:  corev1.ConditionUnknown,
			Reason:  "SinkEmpty",
			Message: "Sink has resolved to empty.",
		},
	}, {
		name: "mark sink nil, secrets, webhook, then sink",
		s: func() *GitHubSourceStatus {
			s := &GitHubSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(nil)
			s.MarkSecrets()
			s.MarkWebhookConfigured()
			s.MarkSink(apis.HTTP("example"))
			return s
		}(),
		condQuery: GitHubSourceConditionReady,
		want: &apis.Condition{
			Type:   GitHubSourceConditionReady,
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
func TestGitHubSource_GetGroupVersionKind(t *testing.T) {
	src := GitHubSource{}
	gvk := src.GetGroupVersionKind()

	if gvk.Kind != "GitHubSource" {
		t.Errorf("Should be 'GitHubSource'.")
	}
}
