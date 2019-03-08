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

// Check that BitBucketSource can be validated and can be defaulted.
var _ runtime.Object = (*BitBucketSource)(nil)

// Check that BitBucketSource implements the Conditions duck type.
var _ = duck.VerifyType(&BitBucketSource{}, &duckv1alpha1.Conditions{})

// BitBucketSourceSpec defines the desired state of BitBucketSource.
// +kubebuilder:categories=all,knative,eventing,sources
type BitBucketSourceSpec struct {
	// ServiceAccountName holds the name of the Kubernetes service account
	// as which the underlying K8s resources should be run. If unspecified
	// this will default to the "default" service account for the namespace
	// in which the BitBucketSource exists.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// OwnerAndRepository is the BitBucket owner/org and repository to
	// receive events from. The repository may be left off to receive
	// events from an entire organization.
	// Examples:
	//  myuser/project
	//  myorganization
	// +kubebuilder:validation:MinLength=1
	OwnerAndRepository string `json:"ownerAndRepository"`

	// EventType is the type of event to receive from BitBucket. These
	// correspond to the event names listed at
	// https://confluence.atlassian.com/bitbucket/event-payloads-740262817.html - e.g.,
	// "pullrequest:created"
	// +kubebuilder:validation:MinItems=1
	// +kubebuilder:validation:Enum=repo:push,repo:fork,repo:updated,repo:commit_comment_created,repo:commit_status_created,repo:commit_status_updated,pullrequest:created,pullrequest:updated,pullrequest:approved,pullrequest:unapproved,pullrequest:fulfilled,pullrequest:rejected,pullrequest:comment_created,pullrequest:comment_updated,pullrequest:comment_deleted,issue:created,issue:updated,issue:comment_created
	EventTypes []string `json:"eventTypes"`

	// ConsumerKey is the Kubernetes secret containing the BitBucket
	// consumer key, i.e., the key generated when creating an Oauth consumer.
	ConsumerKey BitBucketConsumerValue `json:"consumerKey"`

	// ConsumerSecret is the Kubernetes secret containing the BitBucket
	// secret token, i.e., the secret generated when creating an Oauth consumer.
	ConsumerSecret BitBucketConsumerValue `json:"consumerSecret"`

	// Sink is a reference to an object that will resolve to a domain
	// name to use as the sink.
	// +optional
	Sink *corev1.ObjectReference `json:"sink,omitempty"`
}

// BitBucketConsumerValue represents a consumer value secret for the source.
type BitBucketConsumerValue struct {
	// The Secret key to select from.
	SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}

const (
	// BitBucketSourceEventPrefix is what all BitBucket event types get
	// prefixed with when converting to CloudEvent EventType
	BitBucketSourceEventPrefix = "dev.knative.source.bitbucket"
)

const (
	// BitBucketSourceConditionReady has status True when the
	// BitBucketSource is ready to send events.
	BitBucketSourceConditionReady = duckv1alpha1.ConditionReady

	// BitBucketSourceConditionSecretsProvided has status True when the
	// BitBucketSource has valid secret references.
	BitBucketSourceConditionSecretsProvided duckv1alpha1.ConditionType = "SecretsProvided"

	// BitBucketSourceConditionSinkProvided has status True when the
	// BitBucketSource has been configured with a sink target.
	BitBucketSourceConditionSinkProvided duckv1alpha1.ConditionType = "SinkProvided"

	// BitBucketSourceConditionServiceProvided has status True when the
	// BitBucketSource has valid service references.
	BitBucketSourceConditionServiceProvided duckv1alpha1.ConditionType = "ServiceProvided"

	// BitBucketSourceConditionWebHookUUIDProvided has status True when the
	// BitBucketSource has been configured with a webhook.
	BitBucketSourceConditionWebHookUUIDProvided duckv1alpha1.ConditionType = "WebHookUUIDProvided"
)

var bitBucketSourceCondSet = duckv1alpha1.NewLivingConditionSet(
	BitBucketSourceConditionSecretsProvided,
	BitBucketSourceConditionSinkProvided,
	BitBucketSourceConditionServiceProvided,
	BitBucketSourceConditionWebHookUUIDProvided)

// BitBucketSourceStatus defines the observed state of BitBucketSource.
type BitBucketSourceStatus struct {
	// Conditions holds the state of a source at a point in time.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions duckv1alpha1.Conditions `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// WebhookUUIDKey is the UUID of the webhook registered with BitBucket.
	WebhookUUIDKey string `json:"webhookUUIDKey,omitempty"`

	// SinkURI is the current active sink URI that has been configured
	// for the BitBucketSource.
	// +optional
	SinkURI string `json:"sinkUri,omitempty"`
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *BitBucketSourceStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return bitBucketSourceCondSet.Manage(s).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (s *BitBucketSourceStatus) IsReady() bool {
	return bitBucketSourceCondSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *BitBucketSourceStatus) InitializeConditions() {
	bitBucketSourceCondSet.Manage(s).InitializeConditions()
}

// MarkService sets the condition that the source has a service configured.
func (s *BitBucketSourceStatus) MarkService() {
	bitBucketSourceCondSet.Manage(s).MarkTrue(BitBucketSourceConditionServiceProvided)
}

// MarkNoService sets the condition that the source does not have a valid service.
func (s *BitBucketSourceStatus) MarkNoService(reason, messageFormat string, messageA ...interface{}) {
	bitBucketSourceCondSet.Manage(s).MarkFalse(BitBucketSourceConditionServiceProvided, reason, messageFormat, messageA...)
}

// MarkSecrets sets the condition that the source has a valid secret.
func (s *BitBucketSourceStatus) MarkSecrets() {
	bitBucketSourceCondSet.Manage(s).MarkTrue(BitBucketSourceConditionSecretsProvided)
}

// MarkNoSecrets sets the condition that the source does not have a valid secret.
func (s *BitBucketSourceStatus) MarkNoSecrets(reason, messageFormat string, messageA ...interface{}) {
	bitBucketSourceCondSet.Manage(s).MarkFalse(BitBucketSourceConditionSecretsProvided, reason, messageFormat, messageA...)
}

// MarkSink sets the condition that the source has a sink configured.
func (s *BitBucketSourceStatus) MarkSink(uri string) {
	s.SinkURI = uri
	if len(uri) > 0 {
		bitBucketSourceCondSet.Manage(s).MarkTrue(BitBucketSourceConditionSinkProvided)
	} else {
		bitBucketSourceCondSet.Manage(s).MarkUnknown(BitBucketSourceConditionSinkProvided,
			"SinkEmpty", "Sink has resolved to empty.")
	}
}

// MarkNoSink sets the condition that the source does not have a sink configured.
func (s *BitBucketSourceStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	bitBucketSourceCondSet.Manage(s).MarkFalse(BitBucketSourceConditionSinkProvided, reason, messageFormat, messageA...)
}

// MarkWebHook sets the condition that the source has a webhook configured.
func (s *BitBucketSourceStatus) MarkWebHook(uuid string) {
	s.WebhookUUIDKey = uuid
	if len(uuid) > 0 {
		bitBucketSourceCondSet.Manage(s).MarkTrue(BitBucketSourceConditionWebHookUUIDProvided)
	} else {
		bitBucketSourceCondSet.Manage(s).MarkUnknown(BitBucketSourceConditionWebHookUUIDProvided,
			"WebHookUUIDEmpty", "WebHookUUID is empty.")
	}
}

// MarkNoWebHook sets the condition that the source does not have a webhook configured.
func (s *BitBucketSourceStatus) MarkNoWebHook(reason, messageFormat string, messageA ...interface{}) {
	s.WebhookUUIDKey = ""
	bitBucketSourceCondSet.Manage(s).MarkFalse(BitBucketSourceConditionWebHookUUIDProvided, reason, messageFormat, messageA...)
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BitBucketSource is the Schema for the bitbucketsources API.
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:categories=all,knative,eventing,sources
type BitBucketSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   BitBucketSourceSpec   `json:"spec,omitempty"`
	Status BitBucketSourceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BitBucketSourceList contains a list of BitBucketSource.
type BitBucketSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []BitBucketSource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&BitBucketSource{}, &BitBucketSourceList{})
}
