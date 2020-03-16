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
	"fmt"

	"knative.dev/pkg/webhook/resourcesemantics"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

// Check that GitHubSource can be validated and can be defaulted.
var _ runtime.Object = (*GitHubSource)(nil)

var _ resourcesemantics.GenericCRD = (*GitHubSource)(nil)

// GitHubSourceSpec defines the desired state of GitHubSource
// +kubebuilder:categories=all,knative,eventing,sources
type GitHubSourceSpec struct {
	// ServiceAccountName holds the name of the Kubernetes service account
	// as which the underlying K8s resources should be run. If unspecified
	// this will default to the "default" service account for the namespace
	// in which the GitHubSource exists.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// OwnerAndRepository is the GitHub owner/org and repository to
	// receive events from. The repository may be left off to receive
	// events from an entire organization.
	// Examples:
	//  myuser/project
	//  myorganization
	// +kubebuilder:validation:MinLength=1
	OwnerAndRepository string `json:"ownerAndRepository"`

	// EventType is the type of event to receive from GitHub. These
	// correspond to the "Webhook event name" values listed at
	// https://developer.github.com/v3/activity/events/types/ - ie
	// "pull_request"
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Enum=check_suite,commit_comment,create,delete,deployment,deployment_status,fork,gollum,installation,integration_installation,issue_comment,issues,label,member,membership,milestone,organization,org_block,page_build,ping,project_card,project_column,project,public,pull_request,pull_request_review,pull_request_review_comment,push,release,repository,status,team,team_add,watch
	EventTypes []string `json:"eventTypes"`

	// AccessToken is the Kubernetes secret containing the GitHub
	// access token
	AccessToken SecretValueFromSource `json:"accessToken"`

	// SecretToken is the Kubernetes secret containing the GitHub
	// secret token
	SecretToken SecretValueFromSource `json:"secretToken"`

	// Sink is a reference to an object that will resolve to a domain
	// name to use as the sink.
	// +optional
	Sink *duckv1beta1.Destination `json:"sink,omitempty"`

	// API URL if using github enterprise (default https://api.github.com)
	// +optional
	GitHubAPIURL string `json:"githubAPIURL,omitempty"`

	// Secure can be set to true to configure the webhook to use https,
	// or false to use http.  Omitting it relies on the scheme of the
	// Knative Service created (e.g. if auto-TLS is enabled it should
	// do the right thing).
	// +optional
	Secure *bool `json:"secure,omitempty"`
}

// SecretValueFromSource represents the source of a secret value
type SecretValueFromSource struct {
	// The Secret key to select from.
	SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}

const (
	// gitHubEventTypePrefix is what all GitHub event types get
	// prefixed with when converting to CloudEvents.
	gitHubEventTypePrefix = "dev.knative.source.github"

	// gitHubEventSourcePrefix is what all GitHub event sources get
	// prefixed with when converting to CloudEvents.
	gitHubEventSourcePrefix = "https://github.com"
)

// GitHubEventType returns the GitHub CloudEvent type value.
func GitHubEventType(ghEventType string) string {
	return fmt.Sprintf("%s.%s", gitHubEventTypePrefix, ghEventType)
}

// GitHubEventSource returns the GitHub CloudEvent source value.
func GitHubEventSource(ownerAndRepo string) string {
	return fmt.Sprintf("%s/%s", gitHubEventSourcePrefix, ownerAndRepo)
}

const (
	// GitHubSourceConditionReady has status True when the
	// GitHubSource is ready to send events.
	GitHubSourceConditionReady = apis.ConditionReady

	// GitHubSourceConditionSecretsProvided has status True when the
	// GitHubSource has valid secret references
	GitHubSourceConditionSecretsProvided apis.ConditionType = "SecretsProvided"

	// GitHubSourceConditionSinkProvided has status True when the
	// GitHubSource has been configured with a sink target.
	GitHubSourceConditionSinkProvided apis.ConditionType = "SinkProvided"

	// GitHubSourceConditionEventTypesProvided has status True when the
	// GitHubSource has been configured with event types.
	GitHubSourceConditionEventTypesProvided apis.ConditionType = "EventTypeProvided"

	// GitHubServiceconditiondeployed has status True when then
	// GitHubSource Service has been deployed
	//	GitHubServiceConditionDeployed apis.ConditionType = "Deployed"

	// GitHubSourceReconciled has status True when the
	// GitHubSource has been properly reconciled
	GitHub
)

var gitHubSourceCondSet = apis.NewLivingConditionSet(
	GitHubSourceConditionSecretsProvided,
	GitHubSourceConditionSinkProvided)

//	GitHubServiceConditionDeployed)

// GitHubSourceStatus defines the observed state of GitHubSource
type GitHubSourceStatus struct {
	// inherits duck/v1alpha1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1.Status `json:",inline"`

	// WebhookIDKey is the ID of the webhook registered with GitHub
	WebhookIDKey string `json:"webhookIDKey,omitempty"`

	// SinkURI is the current active sink URI that has been configured
	// for the GitHubSource.
	// +optional
	SinkURI string `json:"sinkUri,omitempty"`
}

func (s *GitHubSource) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("GitHubSource")
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *GitHubSourceStatus) GetCondition(t apis.ConditionType) *apis.Condition {
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

// MarkSecrets sets the condition that the source has a valid spec
func (s *GitHubSourceStatus) MarkSecrets() {
	gitHubSourceCondSet.Manage(s).MarkTrue(GitHubSourceConditionSecretsProvided)
}

// MarkNoSecrets sets the condition that the source does not have a valid spec
func (s *GitHubSourceStatus) MarkNoSecrets(reason, messageFormat string, messageA ...interface{}) {
	gitHubSourceCondSet.Manage(s).MarkFalse(GitHubSourceConditionSecretsProvided, reason, messageFormat, messageA...)
}

// MarkSink sets the condition that the source has a sink configured.
func (s *GitHubSourceStatus) MarkSink(uri string) {
	s.SinkURI = uri
	if len(uri) > 0 {
		gitHubSourceCondSet.Manage(s).MarkTrue(GitHubSourceConditionSinkProvided)
	} else {
		gitHubSourceCondSet.Manage(s).MarkUnknown(GitHubSourceConditionSinkProvided,
			"SinkEmpty", "Sink has resolved to empty.")
	}
}

// MarkSinkWarnDeprecated sets the condition that the source has a sink configured and warns ref is deprecated.
func (s *GitHubSourceStatus) MarkSinkWarnRefDeprecated(uri string) {
	s.SinkURI = uri
	if len(uri) > 0 {
		c := apis.Condition{
			Type:     GitHubSourceConditionSinkProvided,
			Status:   corev1.ConditionTrue,
			Severity: apis.ConditionSeverityError,
			Message:  "Using deprecated object ref fields when specifying spec.sink. Update to spec.sink.ref. These will be removed in 0.11.",
		}
		gitHubSourceCondSet.Manage(s).SetCondition(c)
	} else {
		gitHubSourceCondSet.Manage(s).MarkUnknown(GitHubSourceConditionSinkProvided, "SinkEmpty", "Sink has resolved to empty.%s", "")
	}
}

// MarkNoSink sets the condition that the source does not have a sink configured.
func (s *GitHubSourceStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	gitHubSourceCondSet.Manage(s).MarkFalse(GitHubSourceConditionSinkProvided, reason, messageFormat, messageA...)
}

// MarkEventTypes sets the condition that the source has set its event types.
func (s *GitHubSourceStatus) MarkEventTypes() {
	gitHubSourceCondSet.Manage(s).MarkTrue(GitHubSourceConditionEventTypesProvided)
}

// MarkNoEventTypes sets the condition that the source does not its event types configured.
func (s *GitHubSourceStatus) MarkNoEventTypes(reason, messageFormat string, messageA ...interface{}) {
	gitHubSourceCondSet.Manage(s).MarkFalse(GitHubSourceConditionEventTypesProvided, reason, messageFormat, messageA...)
}

// MarkDeployed sets the condition that the source has been deployed.
//func (s *GitHubSourceStatus) MarkServiceDeployed(d *appsv1.Deployment) {
//	if duckv1.DeploymentIsAvailable(&d.Status, false) {
//		gitHubSourceCondSet.Manage(s).MarkTrue(GitHubServiceConditionDeployed)
//	} else {
//		gitHubSourceCondSet.Manage(s).MarkFalse(GitHubServiceConditionDeployed, "ServiceDeploymentUnavailable", "The Deployment '%s' is unavailable.", d.Name)
//	}
//}

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GitHubSource is the Schema for the githubsources API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:categories=all,knative,eventing,sources
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
