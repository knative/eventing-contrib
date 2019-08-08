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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis/duck"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/kmeta"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CouchDbImporter is the Schema for the githubsources API
// +k8s:openapi-gen=true
type CouchDbImporter struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CouchDbImporterSpec   `json:"spec,omitempty"`
	Status CouchDbImporterStatus `json:"status,omitempty"`
}

// Check that CouchDb importer can be validated and can be defaulted.
var _ runtime.Object = (*CouchDbImporter)(nil)

// Check that we can create OwnerReferences to a Configuration.
var _ kmeta.OwnerRefable = (*CouchDbImporter)(nil)

// Check that CouchDbImporter implements the Conditions duck type.
var _ = duck.VerifyType(&CouchDbImporter{}, &duckv1beta1.Conditions{})

const (
	// CouchDbImporterChangesEventType is the CouchDbImporter CloudEvent type for changes.
	CouchDbImporterChangesEventType = "dev.knative.couchdb.changes"
)

// CouchDbImporterSpec defines the desired state of CouchDbImporter
type CouchDbImporterSpec struct {
	// ServiceAccountName holds the name of the Kubernetes service account
	// as which the underlying K8s resources should be run. If unspecified
	// this will default to the "default" service account for the namespace
	// in which the CouchDbImporter exists.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// CouchDbCredentials is the credential to use to access CouchDb.
	// Must be a secret. Only Name and Namespace are used.
	CouchDbCredentials corev1.ObjectReference `json:"credentials,omitempty"`

	// Database is the database to watch for changes
	Database string `json:"database"`

	// Sink is a reference to an object that will resolve to a domain
	// name to use as the sink.
	// +optional
	Sink *corev1.ObjectReference `json:"sink,omitempty"`
}

// GetGroupVersionKind returns the GroupVersionKind.
func (s *CouchDbImporter) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("CouchDbImporter")
}

// CouchDbImporterStatus defines the observed state of CouchDbImporter
type CouchDbImporterStatus struct {
	// inherits duck/v1alpha1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1beta1.Status `json:",inline"`

	// SinkURI is the current active sink URI that has been configured
	// for the CouchDbImporter.
	// +optional
	SinkURI string `json:"sinkUri,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CouchDbImporterList contains a list of CouchDbImporter
type CouchDbImporterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CouchDbImporter `json:"items"`
}

func init() {
	SchemeBuilder.Register(&CouchDbImporter{}, &CouchDbImporterList{})
}
