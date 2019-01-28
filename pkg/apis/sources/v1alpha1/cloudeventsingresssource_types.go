/*
Copyright 2019 The Knative Authors.

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

// Check that KubernetesEventSource can be validated and can be defaulted.
var _ runtime.Object = (*CloudEventsIngressSource)(nil)

// Check that KubernetesEventSource implements the Conditions duck type.
var _ = duck.VerifyType(&CloudEventsIngressSource{}, &duckv1alpha1.Conditions{})

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudEventsIngressSource is the Schema for the CloudEventsIngressSourcees API.
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:categories=all,knative,eventing,sources
type CloudEventsIngressSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CloudEventsIngressSourceSpec   `json:"spec,omitempty"`
	Status CloudEventsIngressSourceStatus `json:"status,omitempty"`
}

const (
	// CloudEventsIngressSourceConditionReady has status True when the CloudEventsIngressSource is ready to send events.
	CloudEventsIngressSourceConditionReady = duckv1alpha1.ConditionReady

	// CloudEventsIngressSourceConditionSinkProvided has status True when the CloudEventsIngressSource has been configured with a sink target.
	CloudEventsIngressSourceConditionSinkProvided duckv1alpha1.ConditionType = "SinkProvided"

	// CloudEventsIngressSourceConditionDeployed has status True when the CloudEventsIngressSource has had it's deployment created.
	CloudEventsIngressSourceConditionDeployed duckv1alpha1.ConditionType = "Deployed"
)

// CloudEventsIngressSourceSpec defines the desired state of CloudEventsIngressSource
type CloudEventsIngressSourceSpec struct {
	// Sink is the URI
	Sink *corev1.ObjectReference `json:"sink,omitempty"`
}

// CloudEventsIngressSourceStatus defines the observed state of CloudEventsIngressSource
type CloudEventsIngressSourceStatus struct {

	//Conditions duckv1alpha1.Conditions `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// inherits duck/v1alpha1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1alpha1.Status `json:",inline"`

	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file
	SinkURI string `json:"sinkURI,omitempty"`
}

var CloudEventsIngressSourceCondSet = duckv1alpha1.NewLivingConditionSet(
	CloudEventsIngressSourceConditionSinkProvided,
	CloudEventsIngressSourceConditionDeployed)

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *CloudEventsIngressSourceStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return CloudEventsIngressSourceCondSet.Manage(s).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (s *CloudEventsIngressSourceStatus) IsReady() bool {
	return CloudEventsIngressSourceCondSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *CloudEventsIngressSourceStatus) InitializeConditions() {
	CloudEventsIngressSourceCondSet.Manage(s).InitializeConditions()
}

// MarkSink sets the condition that the source has a sink configured.
func (s *CloudEventsIngressSourceStatus) MarkSink(uri string) {
	s.SinkURI = uri
	if len(uri) > 0 {
		CloudEventsIngressSourceCondSet.Manage(s).MarkTrue(CloudEventsIngressSourceConditionSinkProvided)
	} else {
		CloudEventsIngressSourceCondSet.Manage(s).MarkUnknown(CloudEventsIngressSourceConditionSinkProvided, "SinkEmpty", "Sink has resolved to empty.")
	}
}

// MarkNoSink sets the condition that the source does not have a sink configured.
func (s *CloudEventsIngressSourceStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	CloudEventsIngressSourceCondSet.Manage(s).MarkFalse(CloudEventsIngressSourceConditionSinkProvided, reason, messageFormat, messageA...)
}

// MarkDeployed sets the condition that the source has been deployed.
func (s *CloudEventsIngressSourceStatus) MarkDeployed() {
	CloudEventsIngressSourceCondSet.Manage(s).MarkTrue(CloudEventsIngressSourceConditionDeployed)
}

// MarkDeploying sets the condition that the source is deploying.
func (s *CloudEventsIngressSourceStatus) MarkDeploying(reason, messageFormat string, messageA ...interface{}) {
	CloudEventsIngressSourceCondSet.Manage(s).MarkUnknown(CloudEventsIngressSourceConditionDeployed, reason, messageFormat, messageA...)
}

// MarkNotDeployed sets the condition that the source has not been deployed.
func (s *CloudEventsIngressSourceStatus) MarkNotDeployed(reason, messageFormat string, messageA ...interface{}) {
	CloudEventsIngressSourceCondSet.Manage(s).MarkFalse(CloudEventsIngressSourceConditionDeployed, reason, messageFormat, messageA...)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CloudEventsIngressSourceList contains a list of CloudEventsIngressSource
type CloudEventsIngressSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CloudEventsIngressSource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CloudEventsIngressSource{}, &CloudEventsIngressSourceList{})
}
