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
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/webhook/resourcesemantics"
)

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// KafkaSource is the Schema for the kafkasources API.
// +k8s:openapi-gen=true
type KafkaSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KafkaSourceSpec   `json:"spec,omitempty"`
	Status KafkaSourceStatus `json:"status,omitempty"`
}

// Check that KafkaSource can be validated and can be defaulted.
var _ runtime.Object = (*KafkaSource)(nil)
var _ resourcesemantics.GenericCRD = (*KafkaSource)(nil)
var _ kmeta.OwnerRefable = (*KafkaSource)(nil)

type KafkaSourceSASLSpec struct {
	Enable bool `json:"enable,omitempty"`

	// User is the Kubernetes secret containing the SASL username.
	// +optional
	User SecretValueFromSource `json:"user,omitempty"`

	// Password is the Kubernetes secret containing the SASL password.
	// +optional
	Password SecretValueFromSource `json:"password,omitempty"`
}

type KafkaSourceTLSSpec struct {
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

type KafkaSourceNetSpec struct {
	SASL KafkaSourceSASLSpec `json:"sasl,omitempty"`
	TLS  KafkaSourceTLSSpec  `json:"tls,omitempty"`
}

type KafkaRequestsSpec struct {
	ResourceCPU    string `json:"cpu,omitempty"`
	ResourceMemory string `json:"memory,omitempty"`
}

type KafkaLimitsSpec struct {
	ResourceCPU    string `json:"cpu,omitempty"`
	ResourceMemory string `json:"memory,omitempty"`
}

type KafkaResourceSpec struct {
	Requests KafkaRequestsSpec `json:"requests,omitempty"`
	Limits   KafkaLimitsSpec   `json:"limits,omitempty"`
}

// KafkaSourceSpec defines the desired state of the KafkaSource.
type KafkaSourceSpec struct {
	// Bootstrap servers are the Kafka servers the consumer will connect to.
	// +required
	BootstrapServers string `json:"bootstrapServers"`

	// Topic topics to consume messages from
	// +required
	Topics string `json:"topics"`

	// ConsumerGroupID is the consumer group ID.
	// +required
	ConsumerGroup string `json:"consumerGroup"`

	Net KafkaSourceNetSpec `json:"net,omitempty"`

	// Sink is a reference to an object that will resolve to a domain name to use as the sink.
	// +optional
	Sink *duckv1.Destination `json:"sink,omitempty"`

	// ServiceAccoutName is the name of the ServiceAccount that will be used to run the Receive
	// Adapter Deployment.
	ServiceAccountName string `json:"serviceAccountName,omitempty"`

	// Resource limits and Request specifications of the Receive Adapter Deployment
	Resources KafkaResourceSpec `json:"resources,omitempty"`
}

const (
	// KafkaEventType is the Kafka CloudEvent type.
	KafkaEventType = "dev.knative.kafka.event"

	KafkaKeyTypeLabel = "kafkasources.sources.knative.dev/key-type"
)

var KafkaKeyTypeAllowed = []string{"string", "int", "float", "byte-array"}

// KafkaEventSource returns the Kafka CloudEvent source.
func KafkaEventSource(namespace, kafkaSourceName, topic string) string {
	return fmt.Sprintf("/apis/v1/namespaces/%s/kafkasources/%s#%s", namespace, kafkaSourceName, topic)
}

// KafkaSourceStatus defines the observed state of KafkaSource.
type KafkaSourceStatus struct {
	// inherits duck/v1 SourceStatus, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last
	//   processed by the controller.
	// * Conditions - the latest available observations of a resource's current
	//   state.
	// * SinkURI - the current active sink URI that has been configured for the
	//   Source.
	duckv1.SourceStatus `json:",inline"`
}

func (s *KafkaSource) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("KafkaSource")
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KafkaSourceList contains a list of KafkaSources.
type KafkaSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KafkaSource `json:"items"`
}
