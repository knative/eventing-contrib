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
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CouchDbSource is the Schema for the githubsources API
// +k8s:openapi-gen=true
type CouchDbSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CouchDbSourceSpec   `json:"spec,omitempty"`
	Status CouchDbSourceStatus `json:"status,omitempty"`
}

// Check that CouchDb source can be validated and can be defaulted.
var _ runtime.Object = (*CouchDbSource)(nil)

// Check that we can create OwnerReferences to a Configuration.
var _ kmeta.OwnerRefable = (*CouchDbSource)(nil)

// Check that CouchDbSource implements the Conditions duck type.
var _ = duck.VerifyType(&CouchDbSource{}, &duckv1beta1.Conditions{})

// FeedType is the type of Feed
type FeedType string

const (
	// CouchDbSourceUpdateEventType is the CouchDbSource CloudEvent type for update.
	CouchDbSourceUpdateEventType = "org.apache.couchdb.document.update"

	// CouchDbSourceDeleteEventType is the CouchDbSource CloudEvent type for deletion.
	CouchDbSourceDeleteEventType = "org.apache.couchdb.document.delete"

	// FeedNormal corresponds to the "normal" feed. The connection to the server
	// is closed after reporting changes.
	FeedNormal = FeedType("normal")

	// FeedContinuous corresponds to the "continuous" feed. The connection to the
	// server stays open after reporting changes.
	FeedContinuous = FeedType("continuous")
)

// CouchDbSourceSpec defines the desired state of CouchDbSource
type CouchDbSourceSpec struct {
	// ServiceAccountName holds the name of the Kubernetes service account
	// as which the underlying K8s resources should be run. If unspecified
	// this will default to the "default" service account for the namespace
	// in which the CouchDbSource exists.
	// +optional
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// CouchDbCredentials is the credential to use to access CouchDb.
	// Must be a secret. Only Name and Namespace are used.
	CouchDbCredentials corev1.ObjectReference `json:"credentials,omitempty"`

	// Feed changes how CouchDB sends the response.
	// More information: https://docs.couchdb.org/en/stable/api/database/changes.html#changes-feeds
	Feed FeedType `json:"feed"`

	// Database is the database to watch for changes
	Database string `json:"database"`

	// Sink is a reference to an object that will resolve to a domain name to use as the sink.
	// +optional
	Sink *duckv1beta1.Destination `json:"sink,omitempty"`
}

// GetGroupVersionKind returns the GroupVersionKind.
func (s *CouchDbSource) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("CouchDbSource")
}

// CouchDbSourceStatus defines the observed state of CouchDbSource
type CouchDbSourceStatus struct {
	// inherits duck/v1alpha1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1beta1.Status `json:",inline"`

	// SinkURI is the current active sink URI that has been configured
	// for the CouchDbSource.
	// +optional
	SinkURI string `json:"sinkUri,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CouchDbSourceList contains a list of CouchDbSource
type CouchDbSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CouchDbSource `json:"items"`
}
