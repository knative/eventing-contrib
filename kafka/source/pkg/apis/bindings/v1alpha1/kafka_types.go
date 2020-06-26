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
	"knative.dev/pkg/webhook/resourcesemantics"
)

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// KafkaBinding is the Schema for the kafkasources API.
// +k8s:openapi-gen=true
type KafkaBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KafkaBindingSpec   `json:"spec,omitempty"`
	Status KafkaBindingStatus `json:"status,omitempty"`
}

// Check that KafkaBinding can be validated and can be defaulted.
var _ runtime.Object = (*KafkaBinding)(nil)
var _ resourcesemantics.GenericCRD = (*KafkaBinding)(nil)
var _ kmeta.OwnerRefable = (*KafkaBinding)(nil)
var _ duckv1.KRShaped = (*KafkaBinding)(nil)

type KafkaSASLSpec struct {
	Enable bool `json:"enable,omitempty"`

	// User is the Kubernetes secret containing the SASL username.
	// +optional
	User SecretValueFromSource `json:"user,omitempty"`

	// Password is the Kubernetes secret containing the SASL password.
	// +optional
	Password SecretValueFromSource `json:"password,omitempty"`
}

type KafkaTLSSpec struct {
	Enable bool `json:"enable,omitempty"`

	// Cert is the Kubernetes secret containing the client certificate.
	// +optional
	Cert SecretValueFromSource `json:"cert,omitempty"`
	// Key is the Kubernetes secret containing the client key.
	// +optional
	Key SecretValueFromSource `json:"key,omitempty"`
	// CACert is the Kubernetes secret containing the server CA cert.
	// +optional
	CACert SecretValueFromSource `json:"caCert,omitempty"`
}

// SecretValueFromSource represents the source of a secret value
type SecretValueFromSource struct {
	// The Secret key to select from.
	SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}

type KafkaNetSpec struct {
	SASL KafkaSASLSpec `json:"sasl,omitempty"`
	TLS  KafkaTLSSpec  `json:"tls,omitempty"`
}

type KafkaAuthSpec struct {
	// Bootstrap servers are the Kafka servers the consumer will connect to.
	// +required
	BootstrapServers []string `json:"bootstrapServers"`

	Net KafkaNetSpec `json:"net,omitempty"`
}

// KafkaBindingSpec defines the desired state of the KafkaBinding.
type KafkaBindingSpec struct {
	duckv1alpha1.BindingSpec `json:",inline"`

	KafkaAuthSpec `json:",inline"`
}

const (
	// KafkaBindingConditionReady is configured to indicate whether the Binding
	// has been configured for resources subject to its runtime contract.
	KafkaBindingConditionReady = apis.ConditionReady
)

// KafkaBindingStatus defines the observed state of KafkaBinding.
type KafkaBindingStatus struct {
	duckv1.Status `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KafkaBindingList contains a list of KafkaBindings.
type KafkaBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KafkaBinding `json:"items"`
}

// GetStatus retrieves the duck status for this resource. Implements the KRShaped interface.
func (k *KafkaBinding) GetStatus() *duckv1.Status {
	return &k.Status.Status
}
