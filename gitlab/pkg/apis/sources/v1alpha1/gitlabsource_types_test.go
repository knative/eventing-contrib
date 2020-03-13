/*
Copyright 2020 The Knative Authors.

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
	"log"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
	corev1 "k8s.io/api/core/v1"
	"knative.dev/pkg/apis"
)

var sinkURL *apis.URL

func init() {
	var err error
	sinkURL, err = apis.ParseURL("http://example.com")
	if err != nil {
		log.Fatal(err)
	}
}

func TestGitLabSourceStatusIsReady(t *testing.T) {
	tests := []struct {
		name string
		s    *GitLabSourceStatus
		want bool
	}{{
		name: "uninitialized",
		s:    &GitLabSourceStatus{},
		want: false,
	}, {
		name: "initialized",
		s: func() *GitLabSourceStatus {
			s := &GitLabSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		want: false,
	}, {
		name: "mark sink",
		s: func() *GitLabSourceStatus {
			s := &GitLabSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(sinkURL)
			return s
		}(),
		want: false,
	}, {
		name: "mark secrets",
		s: func() *GitLabSourceStatus {
			s := &GitLabSourceStatus{}
			s.InitializeConditions()
			s.MarkSecret()
			return s
		}(),
		want: false,
	}, {
		name: "mark sink, secrets, then no sink",
		s: func() *GitLabSourceStatus {
			s := &GitLabSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(sinkURL)
			s.MarkSecret()
			s.MarkNoSink("Testing", "")
			return s
		}(),
		want: false,
	}, {
		name: "mark sink, secrets, then no secrets",
		s: func() *GitLabSourceStatus {
			s := &GitLabSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(sinkURL)
			s.MarkSecret()
			s.MarkNoSecret("Testing", "")
			return s
		}(),
		want: false,
	}, {
		name: "mark sink empty and secrets",
		s: func() *GitLabSourceStatus {
			s := &GitLabSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(&apis.URL{})
			s.MarkSecret()
			return s
		}(),
		want: false,
	}, {
		name: "mark sink empty, secrets, then sink",
		s: func() *GitLabSourceStatus {
			s := &GitLabSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(&apis.URL{})
			s.MarkSecret()
			s.MarkSink(sinkURL)
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

func TestGitLabSourceStatusGetCondition(t *testing.T) {
	tests := []struct {
		name      string
		s         *GitLabSourceStatus
		condQuery apis.ConditionType
		want      *apis.Condition
	}{{
		name:      "uninitialized",
		s:         &GitLabSourceStatus{},
		condQuery: GitLabSourceConditionReady,
		want:      nil,
	}, {
		name: "initialized",
		s: func() *GitLabSourceStatus {
			s := &GitLabSourceStatus{}
			s.InitializeConditions()
			return s
		}(),
		condQuery: GitLabSourceConditionReady,
		want: &apis.Condition{
			Type:   GitLabSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark sink",
		s: func() *GitLabSourceStatus {
			s := &GitLabSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(sinkURL)
			return s
		}(),
		condQuery: GitLabSourceConditionReady,
		want: &apis.Condition{
			Type:   GitLabSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark secrets",
		s: func() *GitLabSourceStatus {
			s := &GitLabSourceStatus{}
			s.InitializeConditions()
			s.MarkSecret()
			return s
		}(),
		condQuery: GitLabSourceConditionReady,
		want: &apis.Condition{
			Type:   GitLabSourceConditionReady,
			Status: corev1.ConditionUnknown,
		},
	}, {
		name: "mark sink, secrets",
		s: func() *GitLabSourceStatus {
			s := &GitLabSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(sinkURL)
			s.MarkSecret()
			return s
		}(),
		condQuery: GitLabSourceConditionReady,
		want: &apis.Condition{
			Type:   GitLabSourceConditionReady,
			Status: corev1.ConditionTrue,
		},
	}, {
		name: "mark sink, secrets, then no sink",
		s: func() *GitLabSourceStatus {
			s := &GitLabSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(sinkURL)
			s.MarkSecret()
			s.MarkNoSink("Testing", "hi%s", "")
			return s
		}(),
		condQuery: GitLabSourceConditionReady,
		want: &apis.Condition{
			Type:    GitLabSourceConditionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "Testing",
			Message: "hi",
		},
	}, {
		name: "mark sink, secrets, then no secrets",
		s: func() *GitLabSourceStatus {
			s := &GitLabSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(sinkURL)
			s.MarkSecret()
			s.MarkNoSecret("Testing", "hi%s", "")
			return s
		}(),
		condQuery: GitLabSourceConditionReady,
		want: &apis.Condition{
			Type:    GitLabSourceConditionReady,
			Status:  corev1.ConditionFalse,
			Reason:  "Testing",
			Message: "hi",
		},
	}, {
		name: "mark sink empty, secrets",
		s: func() *GitLabSourceStatus {
			s := &GitLabSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(&apis.URL{})
			s.MarkSecret()
			return s
		}(),
		condQuery: GitLabSourceConditionReady,
		want: &apis.Condition{
			Type:    GitLabSourceConditionReady,
			Status:  corev1.ConditionUnknown,
			Reason:  "SinkEmpty",
			Message: "Sink has resolved to empty.",
		},
	}, {
		name: "mark sink empty, secrets, then sink",
		s: func() *GitLabSourceStatus {
			s := &GitLabSourceStatus{}
			s.InitializeConditions()
			s.MarkSink(&apis.URL{})
			s.MarkSecret()
			s.MarkSink(sinkURL)
			return s
		}(),
		condQuery: GitLabSourceConditionReady,
		want: &apis.Condition{
			Type:   GitLabSourceConditionReady,
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
func TestGitLabSource_GetGroupVersionKind(t *testing.T) {
	src := GitLabSource{}
	gvk := src.GetGroupVersionKind()

	if gvk.Kind != "GitLabSource" {
		t.Errorf("Should be 'GitLabSource'.")
	}
}
