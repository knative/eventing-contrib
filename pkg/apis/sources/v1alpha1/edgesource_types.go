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
	"k8s.io/apimachinery/pkg/runtime"
)

// EdgeSourceSpec defines the desired state of EdgeSource
type EdgeSourceSpec struct {
	// Service Account used to run jobs. If left out, uses "default"
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Jobs defines what is invoked when the source reconciles.
	// +optional
	Jobs []JobSpec `json:"jobs,omitempty"`

	// Image is the image of image of the container to pass to StartJob
	// +optional
	Image string `json:"image,omitempty"`

	// TODO: Args are passed to the ContainerSpec as they are.
	// Args []string `json:"args,omitempty"`

	// Sink is a reference to an object that will resolve to a domain name to use as the sink.
	// +optional
	Sink *corev1.ObjectReference `json:"sink,omitempty"`
}

type JobType string

const (
	// EdgeJobStart is the job type for running on "start" reconciliation.
	EdgeJobStart JobType = "Start"

	// EdgeJobStop is the job type for running on "stop" reconciliation.
	EdgeJobStop JobType = "Stop"
)

type JobSpec struct {
	// Image is the container the job will run.
	// +optional
	Image string `json:"image,omitempty"`

	// Type defines on which event the job is run.
	// TODO: support `Update`
	// +kubebuilder:validation:Enum=Start,Stop
	Type JobType `json:"type,omitempty"`

	// Args are passed to the Job as they are.
	Args []string `json:"args,omitempty"`

	// TODO this needs secrets and others
}

// GetJob returns the job that matches the given type, or nil.
func (s *EdgeSourceSpec) GetJob(jt JobType) *JobSpec {
	if s == nil {
		return nil
	}
	for _, j := range s.Jobs {
		if j.Type == jt {
			return &j
		}
	}
	return nil
}

const (
	// ContainerSourceConditionReady has status True when the ContainerSource is ready to send events.
	EdgeConditionReady = duckv1alpha1.ConditionReady

	// EdgeConditionSinkProvided has status True when the EdgeSource has been configured with a sink target.
	EdgeConditionSinkProvided duckv1alpha1.ConditionType = "SinkProvided"

	// EdgeConditionJob describes the status of the Start and Stop jobs related to this source.
	EdgeConditionJob duckv1alpha1.ConditionType = "Job"
)

var edgeCondSet = duckv1alpha1.NewLivingConditionSet(
	EdgeConditionSinkProvided,
	EdgeConditionJob)

// EdgeSourceStatus defines the observed state of EdgeSource
type EdgeSourceStatus struct {
	// Conditions holds the state of a source at a point in time.
	// +optional
	// +patchMergeKey=type
	// +patchStrategy=merge
	Conditions duckv1alpha1.Conditions `json:"conditions,omitempty" patchStrategy:"merge" patchMergeKey:"type"`

	// SinkURI is the current active sink URI that has been configured for the ContainerSource.
	// +optional
	SinkURI string `json:"sinkUri,omitempty"`

	// JobContext is what the Job operation returns and holds enough information
	// for the source to update or stop the source.
	// This is specific to each Source. Opaque to platform, only consumed
	// by the actual Job.
	// NOTE: experimental field.
	JobContext *runtime.RawExtension `json:"jobContext,omitempty"`
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *EdgeSourceStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return edgeCondSet.Manage(s).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (s *EdgeSourceStatus) IsReady() bool {
	return edgeCondSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *EdgeSourceStatus) InitializeConditions() {
	edgeCondSet.Manage(s).InitializeConditions()
}

// MarSink sets the condition that the source has a sink configured.
func (s *EdgeSourceStatus) MarkSink(uri string) {
	s.SinkURI = uri
	if len(uri) > 0 {
		containerCondSet.Manage(s).MarkTrue(EdgeConditionSinkProvided)
	} else {
		containerCondSet.Manage(s).MarkUnknown(EdgeConditionSinkProvided, "SinkEmpty", "Sink has resolved to empty.%s", "")
	}
}

// MarkNoSink sets the condition that the source does not have a sink configured.
func (s *EdgeSourceStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	containerCondSet.Manage(s).MarkFalse(EdgeConditionSinkProvided, reason, messageFormat, messageA...)
}

// MarkJobStarted sets the condition that the source job has started.
func (s *EdgeSourceStatus) MarkJobStarted() {
	containerCondSet.Manage(s).MarkTrue(EdgeConditionJob)
}

// MarkJobStarting sets the condition that the source job is starting.
func (s *EdgeSourceStatus) MarkJobStarting(reason, messageFormat string, messageA ...interface{}) {
	containerCondSet.Manage(s).MarkUnknown(EdgeConditionJob, reason, messageFormat, messageA...)
}

// MarkJobFailed sets the condition that the source job has failed.
func (s *EdgeSourceStatus) MarkJobFailed(reason, messageFormat string, messageA ...interface{}) {
	containerCondSet.Manage(s).MarkFalse(EdgeConditionJob, reason, messageFormat, messageA...)
}

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EdgeSource is the Schema for the edgesources API
// +k8s:openapi-gen=true
type EdgeSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   EdgeSourceSpec   `json:"spec,omitempty"`
	Status EdgeSourceStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// EdgeSourceList contains a list of EdgeSource
type EdgeSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []EdgeSource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&EdgeSource{}, &EdgeSourceList{})
}
