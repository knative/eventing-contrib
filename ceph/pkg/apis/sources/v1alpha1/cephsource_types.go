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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/webhook/resourcesemantics"
)

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
type CephSource struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec holds the desired state of the CephSource (from the client).
	Spec CephSourceSpec `json:"spec"`

	// Status communicates the observed state of the CephSource (from the controller).
	// +optional
	Status CephSourceStatus `json:"status,omitempty"`
}

// GetGroupVersionKind returns the GroupVersionKind.
func (*CephSource) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("CephSource")
}

var (
	// Check that CephSource can be validated and defaulted.
	_ apis.Validatable = (*CephSource)(nil)
	_ apis.Defaultable = (*CephSource)(nil)
	// Check that we can create OwnerReferences to a CephSource.
	_ kmeta.OwnerRefable = (*CephSource)(nil)
	// Check that CephSource is a runtime.Object.
	_ runtime.Object = (*CephSource)(nil)
	// Check that CephSource satisfies resourcesemantics.GenericCRD.
	_ resourcesemantics.GenericCRD = (*CephSource)(nil)
	// Check that CephSource implements the Conditions duck type.
	_ = duck.VerifyType(&CephSource{}, &duckv1.Conditions{})
	// Check that the type conforms to the duck Knative Resource shape.
	_ duckv1.KRShaped = (*CephSource)(nil)
)

// CephSourceSpec holds the desired state of the CephSource (from the client).
type CephSourceSpec struct {
	// inherits duck/v1 SourceSpec, which currently provides:
	// * Sink - a reference to an object that will resolve to a domain name or
	//   a URI directly to use as the sink.
	// * CloudEventOverrides - defines overrides to control the output format
	//   and modifications of the event sent to the sink.
	duckv1.SourceSpec `json:",inline"`

	// ServiceAccountName holds the name of the Kubernetes service account
	// as which the underlying K8s resources should be run. If unspecified
	// this will default to the "default" service account for the namespace
	// in which the CephSource exists.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
}

const (
	// CephSourceConditionReady is set when the revision is starting to materialize
	// runtime resources, and becomes true when those resources are ready.
	CephSourceConditionReady = apis.ConditionReady
)

// CephSourceStatus communicates the observed state of the CephSource (from the controller).
type CephSourceStatus struct {
	// inherits duck/v1 SourceStatus, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last
	//   processed by the controller.
	// * Conditions - the latest available observations of a resource's current
	//   state.
	// * SinkURI - the current active sink URI that has been configured for the
	//   Source.
	duckv1.SourceStatus `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CephSourceList is a list of CephSource resources
type CephSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []CephSource `json:"items"`
}

// GetStatus retrieves the status of the resource. Implements the KRShaped interface.
func (cs *CephSource) GetStatus() *duckv1.Status {
	return &cs.Status.Status
}
