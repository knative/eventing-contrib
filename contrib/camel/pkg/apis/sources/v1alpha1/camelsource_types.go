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

// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CamelSource is the Schema for the camelsources API
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:categories=all,knative,eventing,sources
type CamelSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CamelSourceSpec   `json:"spec,omitempty"`
	Status CamelSourceStatus `json:"status,omitempty"`
}

// Check that CamelSource can be validated and can be defaulted.
var _ runtime.Object = (*CamelSource)(nil)

// Check that CamelSource implements the Conditions duck type.
var _ = duck.VerifyType(&CamelSource{}, &duckv1alpha1.Conditions{})

const (
	// CamelSourceConditionReady has status True when the CamelSource is ready to send events.
	CamelConditionReady = duckv1alpha1.ConditionReady

	// CamelConditionSinkProvided has status True when the CamelSource has been configured with a sink target.
	CamelConditionSinkProvided duckv1alpha1.ConditionType = "SinkProvided"

	// CamelConditionDeployed has status True when the CamelSource has had it's deployment created.
	CamelConditionDeployed duckv1alpha1.ConditionType = "Deployed"
)

var camelCondSet = duckv1alpha1.NewLivingConditionSet(
	CamelConditionSinkProvided,
	CamelConditionDeployed,
)

// CamelSourceSpec defines the desired state of CamelSource
type CamelSourceSpec struct {
	// Source is the reference to the integration flow to run.
	Source CamelSourceOriginSpec `json:"source"`

	// ServiceAccountName is the name of the ServiceAccount to use to run this
	// source.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Image is an optional base image used to run the source.
	// +optional
	Image string `json:"image,omitempty"`

	// Sink is a reference to an object that will resolve to a domain name to use as the sink.
	// +optional
	Sink *corev1.ObjectReference `json:"sink,omitempty"`
}

// CamelSourceOriginSpec is the integration flow to run
type CamelSourceOriginSpec struct {
	// Component is a kind of source that directly references a Camel component
	Component *CamelSourceOriginComponentSpec `json:"component,omitempty"`
	// Other kind of sources, such as Camel K integrations will be added here
}

type CamelSourceOriginComponentSpec struct {
	// URI is a Camel component URI to use as starting point (e.g. "timer:tick?period=2s")
	// +kubebuilder:validation:MinLength=1
	URI        string            `json:"uri,omitempty"`
	Properties map[string]string `json:"properties,omitempty"`
}

// CamelSourceStatus defines the observed state of CamelSource
type CamelSourceStatus struct {
	// inherits duck/v1alpha1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1alpha1.Status `json:",inline"`

	// SinkURI is the current active sink URI that has been configured for the CamelSource.
	// +optional
	SinkURI string `json:"sinkUri,omitempty"`
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *CamelSourceStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return camelCondSet.Manage(s).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (s *CamelSourceStatus) IsReady() bool {
	return camelCondSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *CamelSourceStatus) InitializeConditions() {
	camelCondSet.Manage(s).InitializeConditions()
}

// MarSink sets the condition that the source has a sink configured.
func (s *CamelSourceStatus) MarkSink(uri string) {
	s.SinkURI = uri
	if len(uri) > 0 {
		camelCondSet.Manage(s).MarkTrue(CamelConditionSinkProvided)
	} else {
		camelCondSet.Manage(s).MarkUnknown(CamelConditionSinkProvided, "SinkEmpty", "Sink has resolved to empty.")
	}
}

// MarkNoSink sets the condition that the source does not have a sink configured.
func (s *CamelSourceStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	camelCondSet.Manage(s).MarkFalse(CamelConditionSinkProvided, reason, messageFormat, messageA...)
}

// MarkDeployed sets the condition that the source has been deployed.
func (s *CamelSourceStatus) MarkDeployed() {
	camelCondSet.Manage(s).MarkTrue(CamelConditionDeployed)
}

// MarkDeploying sets the condition that the source is deploying.
func (s *CamelSourceStatus) MarkDeploying(reason, messageFormat string, messageA ...interface{}) {
	camelCondSet.Manage(s).MarkUnknown(CamelConditionDeployed, reason, messageFormat, messageA...)
}

// MarkNotDeployed sets the condition that the source has not been deployed.
func (s *CamelSourceStatus) MarkNotDeployed(reason, messageFormat string, messageA ...interface{}) {
	camelCondSet.Manage(s).MarkFalse(CamelConditionDeployed, reason, messageFormat, messageA...)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CamelSourceList contains a list of CamelSource
type CamelSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CamelSource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CamelSource{}, &CamelSourceList{})
}
