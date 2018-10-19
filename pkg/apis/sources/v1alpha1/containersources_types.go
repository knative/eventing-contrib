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
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Important: Run "make" to regenerate code after modifying this file
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// ContainerSourcesSpec defines the desired state of ContainerSources
type ContainerSourcesSpec struct {
	// Image is the image to run inside of the container.
	// +kubebuilder:validation:MinLength=1
	Image string `json:"image,omitempty"`

	// Args are passed to the ContainerSpec as they are.
	Args []string `json:"args,omitempty"`

	// Sink is a reference to an object that will resolve to a domain name to use as the sink.
	// +optional
	Sink *corev1.ObjectReference `json:"sink,omitempty"`
}

const (
	// ContainerSourceConditionReady has status True when the ContainerSource is ready to send events.
	ContainerConditionReady = duckv1alpha1.ConditionReady

	// ContainerConditionSinkProvided has status True when the ContainerSource has been configured with a sink target.
	ContainerConditionSinkProvided duckv1alpha1.ConditionType = "SinkProvided"
)

var containerCondSet = duckv1alpha1.NewLivingConditionSet(ContainerConditionSinkProvided)

// ContainerSourcesStatus defines the observed state of ContainerSources
type ContainerSourcesStatus struct {
	// Conditions holds the state of a source at a point in time.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions duckv1alpha1.Conditions `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// TODO(n3wscott): there was talk of adding sink to the status. revisit this later.

	// SinkURI is the current active sink URI that has been configured for the ContainerSource.
	SinkURI string `json:"sinkUri,omitempty"`
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *ContainerSourcesStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return containerCondSet.Manage(s).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (s *ContainerSourcesStatus) IsReady() bool {
	return containerCondSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *ContainerSourcesStatus) InitializeConditions() {
	containerCondSet.Manage(s).InitializeConditions()
}

// MarSink sets the condition that the source has a sink configured.
func (s *ContainerSourcesStatus) MarkSink(uri string) {
	s.SinkURI = uri
	if len(uri) > 0 {
		containerCondSet.Manage(s).MarkTrue(ContainerConditionSinkProvided)
	} else {
		containerCondSet.Manage(s).MarkUnknown(ContainerConditionSinkProvided, "SinkEmpty", "Sink has resolved to empty.")
	}
}

// MarkNoSink sets the condition that the source does not have a sink configured.
func (s *ContainerSourcesStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	containerCondSet.Manage(s).MarkFalse(ContainerConditionSinkProvided, reason, messageFormat, messageA)
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ContainerSources is the Schema for the containersources API
// +k8s:openapi-gen=true
type ContainerSources struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   ContainerSourcesSpec   `json:"spec,omitempty"`
	Status ContainerSourcesStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// ContainerSourcesList contains a list of ContainerSources
type ContainerSourcesList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []ContainerSources `json:"items"`
}

func init() {
	SchemeBuilder.Register(&ContainerSources{}, &ContainerSourcesList{})
}
