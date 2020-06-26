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

package v1beta1

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	bindingsv1beta1 "knative.dev/eventing-contrib/kafka/source/pkg/apis/bindings/v1beta1"
	"knative.dev/pkg/apis"
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
var _ apis.Defaultable = (*KafkaSource)(nil)
var _ apis.Validatable = (*KafkaSource)(nil)
var _ duckv1.KRShaped = (*KafkaSource)(nil)

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
	bindingsv1beta1.KafkaAuthSpec `json:",inline"`

	// Topic topics to consume messages from
	// +required
	Topics []string `json:"topics"`

	// ConsumerGroupID is the consumer group ID.
	// +optional
	ConsumerGroup string `json:"consumerGroup,omitempty"`

	// inherits duck/v1 SourceSpec, which currently provides:
	// * Sink - a reference to an object that will resolve to a domain name or
	//   a URI directly to use as the sink.
	// * CloudEventOverrides - defines overrides to control the output format
	//   and modifications of the event sent to the sink.
	duckv1.SourceSpec `json:",inline"`
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

func (*KafkaSource) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("KafkaSource")
}

// GetStatus retrieves the duck status for this resource. Implements the KRShaped interface.
func (k *KafkaSource) GetStatus() *duckv1.Status {
	return &k.Status.Status
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KafkaSourceList contains a list of KafkaSources.
type KafkaSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KafkaSource `json:"items"`
}
