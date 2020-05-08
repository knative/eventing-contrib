/*
Copyright 2020 The Knative Authors

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
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/kmeta"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:defaulter-gen=true

// GitLabBinding describes a Binding that is also a Source.
// The `sink` (from the Source duck) is resolved to a URL and
// then projected into the `subject` by augmenting the runtime
// contract of the referenced containers to have a `K_SINK`
// environment variable holding the endpoint to which to send
// cloud events.
type GitLabBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   GitLabBindingSpec   `json:"spec"`
	Status GitLabBindingStatus `json:"status"`
}

// Check the interfaces that GitLabBinding should be implementing.
var (
	_ runtime.Object     = (*GitLabBinding)(nil)
	_ kmeta.OwnerRefable = (*GitLabBinding)(nil)
	_ apis.Validatable   = (*GitLabBinding)(nil)
	_ apis.Defaultable   = (*GitLabBinding)(nil)
	_ apis.HasSpec       = (*GitLabBinding)(nil)
)

// GitLabBindingSpec holds the desired state of the GitLabBinding (from the client).
type GitLabBindingSpec struct {
	duckv1alpha1.BindingSpec `json:",inline"`

	// AccessToken is the Kubernetes secret containing the GitLab
	// access token
	AccessToken SecretValueFromSource `json:"accessToken"`
}

// SecretValueFromSource represents the source of a secret value
type SecretValueFromSource struct {
	// The Secret key to select from.
	SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}

const (
	// GitLabBindingConditionReady is configured to indicate whether the Binding
	// has been configured for resources subject to its runtime contract.
	GitLabBindingConditionReady = apis.ConditionReady
)

// GitLabBindingStatus communicates the observed state of the GitLabBinding (from the controller).
type GitLabBindingStatus struct {
	duckv1.SourceStatus `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// GitLabBindingList contains a list of GitLabBinding
type GitLabBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []GitLabBinding `json:"items"`
}
