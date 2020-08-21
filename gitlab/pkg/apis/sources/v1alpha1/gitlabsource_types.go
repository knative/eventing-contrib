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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

var (

	// Check that GitLabSource implements the Conditions duck type.
	_ = duck.VerifyType(&GitLabSource{}, &duckv1.Conditions{})

	_ apis.Validatable = (*GitLabSource)(nil)
	_ apis.Defaultable = (*GitLabSource)(nil)
	_ runtime.Object   = (*GitLabSource)(nil)
	_ duckv1.KRShaped  = (*GitLabSource)(nil)
)

// GitLabSourceSpec defines the desired state of GitLabSource
// +kubebuilder:categories=all,knative,eventing,sources
type GitLabSourceSpec struct {
	// ServiceAccountName holds the name of the Kubernetes service account
	// as which the underlying K8s resources should be run. If unspecified
	// this will default to the "default" service account for the namespace
	// in which the GitLabSource exists.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// ProjectUrl is the url of the GitLab project for which we are interested
	// to receive events from.
	// Examples:
	//   https://gitlab.com/gitlab-org/gitlab-foss
	// +kubebuilder:validation:MinLength=1
	ProjectUrl string `json:"projectUrl"`

	// EventType is the type of event to receive from GitLab. These
	// correspond to supported events to the add project hook
	// https://docs.gitlab.com/ee/api/projects.html#add-project-hook
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Enum=push_events,push_events_branch_filter,issues_events,confidential_issues_events,merge_requests_events,tag_push_events,note_events,job_events,pipeline_events,wiki_page_events
	EventTypes []string `json:"eventTypes"`

	// AccessToken is the Kubernetes secret containing the GitLab
	// access token
	AccessToken SecretValueFromSource `json:"accessToken"`

	// SecretToken is the Kubernetes secret containing the GitLab
	// secret token
	SecretToken SecretValueFromSource `json:"secretToken"`

	// SslVerify if true configure webhook so the ssl verification is done when triggering the hook
	SslVerify bool `json:"sslverify,omitempty"`

	// Sink is a reference to an object that will resolve to a domain
	// name to use as the sink.
	// +optional
	Sink *duckv1.Destination `json:"sink,omitempty"`
}

// SecretValueFromSource represents the source of a secret value
type SecretValueFromSource struct {
	// The Secret key to select from.
	SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}

// GitLabSourceStatus defines the observed state of GitLabSource
type GitLabSourceStatus struct {
	// inherits duck/v1 SourceStatus, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last
	//   processed by the controller.
	// * Conditions - the latest available observations of a resource's current
	//   state.
	// * SinkURI - the current active sink URI that has been configured for the
	//   Source.
	duckv1.SourceStatus `json:",inline"`

	// ID of the project hook registered with GitLab
	Id string `json:"Id,omitempty"`
}

const (
	// GitLabSourceConditionReady has status True when the
	// GitLabSource is ready to send events.
	GitLabSourceConditionReady = apis.ConditionReady

	// GitLabSourceConditionSinkProvided has status True when the
	// GitLabSource has been configured with a sink target.
	GitLabSourceConditionSinkProvided apis.ConditionType = "SinkProvided"

	// GitLabSourceConditionSecretProvided has status True when the
	// GitLabSource can read the secret containing GitLab tokens.
	GitLabSourceConditionSecretProvided apis.ConditionType = "SecretProvided"

	// GitLabSourceConditionWebhookConfigured has a status True when the
	// GitLabSource has been configured with a webhook.
	GitLabSourceConditionWebhookConfigured apis.ConditionType = "WebhookConfigured"

	// GitLabSourceConditionDeployed has status True when the
	// GitLabSource's receive adapter has been successfully deployed.
	GitLabSourceConditionDeployed apis.ConditionType = "Deployed"
)

var gitLabSourceCondSet = apis.NewLivingConditionSet(
	GitLabSourceConditionSecretProvided,
	GitLabSourceConditionSinkProvided,
	GitLabSourceConditionWebhookConfigured,
	GitLabSourceConditionDeployed)

// GetGroupVersionKind returns a GitLabSource GVK.
func (*GitLabSource) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("GitLabSource")
}

// GetConditionSet retrieves the condition set for this resource. Implements the KRShaped interface.
func (*GitLabSource) GetConditionSet() apis.ConditionSet {
	return gitLabSourceCondSet
}

// GetStatus retrieves the duck status for this resource. Implements the KRShaped interface.
func (g *GitLabSource) GetStatus() *duckv1.Status {
	return &g.Status.Status
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *GitLabSourceStatus) GetCondition(t apis.ConditionType) *apis.Condition {
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

// MarkSink sets the SinkProvided condition to True.
func (s *GitLabSourceStatus) MarkSink(uri *apis.URL) {
	s.SinkURI = uri
	if uri.IsEmpty() {
		gitLabSourceCondSet.Manage(s).MarkUnknown(GitLabSourceConditionSinkProvided, "SinkEmpty", "Sink has resolved to empty.")
	} else {
		gitLabSourceCondSet.Manage(s).MarkTrue(GitLabSourceConditionSinkProvided)
	}
}

// MarkNoSink sets the SinkProvided condition to False with the given reason and message.
func (s *GitLabSourceStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	gitLabSourceCondSet.Manage(s).MarkFalse(GitLabSourceConditionSinkProvided, reason, messageFormat, messageA...)
}

// MarkSecret sets the SecretProvided condition to True.
func (s *GitLabSourceStatus) MarkSecret() {
	gitLabSourceCondSet.Manage(s).MarkTrue(GitLabSourceConditionSecretProvided)
}

// MarkNoSecret sets the SecretProvided condition to False with the given reason and message.
func (s *GitLabSourceStatus) MarkNoSecret(reason, messageFormat string, messageA ...interface{}) {
	gitLabSourceCondSet.Manage(s).MarkFalse(GitLabSourceConditionSecretProvided, reason, messageFormat, messageA...)
}

// MarkWebhook sets the WebhookConfigured condition to True.
func (s *GitLabSourceStatus) MarkWebhook() {
	gitLabSourceCondSet.Manage(s).MarkTrue(GitLabSourceConditionWebhookConfigured)
}

// MarkNoWebhook sets the WebhookConfigured condition to False with the given reason and message.
func (s *GitLabSourceStatus) MarkNoWebhook(reason, messageFormat string, messageA ...interface{}) {
	gitLabSourceCondSet.Manage(s).MarkFalse(GitLabSourceConditionWebhookConfigured, reason, messageFormat, messageA...)
}

// MarkWebhook sets the Deployed condition to True.
func (s *GitLabSourceStatus) MarkDeployed() {
	gitLabSourceCondSet.Manage(s).MarkTrue(GitLabSourceConditionDeployed)
}

// MarkNotDeployed sets the Deployed condition to False with the given reason and message.
func (s *GitLabSourceStatus) MarkNotDeployed(reason, messageFormat string, messageA ...interface{}) {
	gitLabSourceCondSet.Manage(s).MarkFalse(GitLabSourceConditionDeployed, reason, messageFormat, messageA...)
}

// +genclient
// +genreconciler
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
