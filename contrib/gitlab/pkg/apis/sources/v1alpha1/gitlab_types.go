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
	"github.com/knative/pkg/apis/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// Check that GitLabSource can be validated and can be defaulted.
var _ runtime.Object = (*GitLabSource)(nil)

// Check that GitLabSource implements the Conditions duck type.
var _ = duck.VerifyType(&GitLabSource{}, &duckv1alpha1.Conditions{})

// GitLabSourceSpec defines the desired state of GitLabSource
// +kubebuilder:categories=all,knative,eventing,sources
type GitLabSourceSpec struct {
	// ServiceAccountName holds the name of the Kubernetes service account
	// as which the underlying K8s resources should be run. If unspecified
	// this will default to the "default" service account for the namespace
	// in which the GitLabSource exists.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// ProjectURL is the url of the GitLab project for which we are interested
	// receive events from.
	// Examples:
	//   https://gitlab.com/triggermesh/gitlabsource
	// +kubebuilder:validation:MinLength=1
	ProjectURL string `json:"projectURL"`

	// EventType is the type of event to receive from GitLab. These
	// correspond to the "Webhook event name" values listed at
	// https://docs.gitlab.com/ee/api/events.html
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Enum=push_events,push_events_branch_filter,issues_events,confidential_issues_events,merge_requests_events,tag_push_events,note_events,job_events,pipeline_events,wiki_page_events
	EventTypes []string `json:"eventTypes"`

	// AccessToken is the Kubernetes secret containing the GitLab
	// access token
	AccessToken SecretValueFromSource `json:"accessToken"`

	// SecretToken is the Kubernetes secret containing the GitLab
	// secret token
	SecretToken SecretValueFromSource `json:"secretToken"`

	// Sink is a reference to an object that will resolve to a domain
	// name to use as the sink.
	// +optional
	Sink *corev1.ObjectReference `json:"sink,omitempty"`

	// Secure can be set to true to configure the webhook to use https.
	// +optional
	Secure bool `json:"secure,omitempty"`
}

// SecretValueFromSource represents the source of a secret value
type SecretValueFromSource struct {
	// The Secret key to select from.
	SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}

const (
	// GitLabSourceEventPrefix is what all GitLab event types get
	// prefixed with when converting to CloudEvent EventType
	GitLabSourceEventPrefix = "dev.knative.source.gitlab"
)

const (
	// GitLabSourceConditionReady has status True when the
	// GitLabSource is ready to send events.
	GitLabSourceConditionReady = duckv1alpha1.ConditionReady

	// GitLabSourceConditionSecretsProvided has status True when the
	// GitLabSource has valid secret references
	GitLabSourceConditionSecretsProvided duckv1alpha1.ConditionType = "SecretsProvided"

	// GitLabSourceConditionSinkProvided has status True when the
	// GitLabSource has been configured with a sink target.
	GitLabSourceConditionSinkProvided duckv1alpha1.ConditionType = "SinkProvided"
)

var gitLabSourceCondSet = duckv1alpha1.NewLivingConditionSet(
	GitLabSourceConditionSecretsProvided,
	GitLabSourceConditionSinkProvided)

// GitLabSourceStatus defines the observed state of GitLabSource
type GitLabSourceStatus struct {
	// inherits duck/v1alpha1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1alpha1.Status `json:",inline"`

	// WebhookIDKey is the ID of the webhook registered with GitLab
	WebhookIDKey string `json:"webhookIDKey,omitempty"`

	// SinkURI is the current active sink URI that has been configured
	// for the GitLabSource.
	// +optional
	SinkURI string `json:"sinkUri,omitempty"`
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *GitLabSourceStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return gitLabSourceCondSet.Manage(s).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (s *GitLabSourceStatus) IsReady() bool {
	return gitLabSourceCondSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *GitLabSourceStatus) InitializeConditions() {
	gitLabSourceCondSet.Manage(s).InitializeConditions()
}

// MarkSecrets sets the condition that the source has a valid spec
func (s *GitLabSourceStatus) MarkSecrets() {
	gitLabSourceCondSet.Manage(s).MarkTrue(GitLabSourceConditionSecretsProvided)
}

// MarkNoSecrets sets the condition that the source does not have a valid spec
func (s *GitLabSourceStatus) MarkNoSecrets(reason, messageFormat string, messageA ...interface{}) {
	gitLabSourceCondSet.Manage(s).MarkFalse(GitLabSourceConditionSecretsProvided, reason, messageFormat, messageA...)
}

// MarkSink sets the condition that the source has a sink configured.
func (s *GitLabSourceStatus) MarkSink(uri string) {
	s.SinkURI = uri
	if len(uri) > 0 {
		gitLabSourceCondSet.Manage(s).MarkTrue(GitLabSourceConditionSinkProvided)
	} else {
		gitLabSourceCondSet.Manage(s).MarkUnknown(GitLabSourceConditionSinkProvided,
			"SinkEmpty", "Sink has resolved to empty.")
	}
}

// MarkNoSink sets the condition that the source does not have a sink configured.
func (s *GitLabSourceStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	gitLabSourceCondSet.Manage(s).MarkFalse(GitLabSourceConditionSinkProvided, reason, messageFormat, messageA...)
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GitLabSource is the Schema for the gitlabsources API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:categories=all,knative,eventing,sources
type GitLabSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GitLabSourceSpec   `json:"spec,omitempty"`
	Status GitLabSourceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GitLabSourceList contains a list of GitLabSource
type GitLabSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GitLabSource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&GitLabSource{}, &GitLabSourceList{})
}
