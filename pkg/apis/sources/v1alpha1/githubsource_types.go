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
	"github.com/knative/pkg/apis/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// Check that GitHubSource can be validated and can be defaulted.
var _ runtime.Object = (*GitHubSource)(nil)

// Check that GitHubSource implements the Conditions duck type.
var _ = duck.VerifyType(&GitHubSource{}, &duckv1alpha1.Conditions{})

// GitHubSourceSpec defines the desired state of GitHubSource
type GitHubSourceSpec struct {
	// Sink is a reference to an object that will resolve to a domain
	// name to use as the sink.
	// +optional
	Sink *corev1.ObjectReference `json:"sink,omitempty"`
}

const (
	// GitHubSourceConditionReady has status True when the
	// GitHubSource is ready to send events.
	GitHubSourceConditionReady = duckv1alpha1.ConditionReady

	// GitHubSourceConditionSinkProvided has status True when the
	// GitHubSource has been configured with a sink target.
	GitHubSourceConditionSinkProvided duckv1alpha1.ConditionType = "SinkProvided"
)

var gitHubSourceCondSet = duckv1alpha1.NewLivingConditionSet(
	GitHubSourceConditionSinkProvided)

// GitHubSourceStatus defines the observed state of GitHubSource
type GitHubSourceStatus struct {
	// Conditions holds the state of a source at a point in time.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions duckv1alpha1.Conditions `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// SinkURI is the current active sink URI that has been configured
	// for the GitHubSource.
	// +optional
	SinkURI string `json:"sinkUri,omitempty"`
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *GitHubSourceStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return gitHubSourceCondSet.Manage(s).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (s *GitHubSourceStatus) IsReady() bool {
	return gitHubSourceCondSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *GitHubSourceStatus) InitializeConditions() {
	gitHubSourceCondSet.Manage(s).InitializeConditions()
}

// MarkSink sets the condition that the source has a sink configured.
func (s *GitHubSourceStatus) MarkSink(uri string) {
	s.SinkURI = uri
	if len(uri) > 0 {
		gitHubSourceCondSet.Manage(s).MarkTrue(GitHubSourceConditionSinkProvided)
	} else {
		gitHubSourceCondSet.Manage(s).MarkUnknown(GitHubSourceConditionSinkProvided, "SinkEmpty", "Sink has resolved to empty.%s", "")
	}
}

// MarkNoSink sets the condition that the source does not have a sink configured.
func (s *GitHubSourceStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	gitHubSourceCondSet.Manage(s).MarkFalse(GitHubSourceConditionSinkProvided, reason, messageFormat, messageA...)
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GitHubSource is the Schema for the githubsources API
// +k8s:openapi-gen=true
type GitHubSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GitHubSourceSpec   `json:"spec,omitempty"`
	Status GitHubSourceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GitHubSourceList contains a list of GitHubSource
type GitHubSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GitHubSource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GitHubSource{}, &GitHubSourceList{})
}
