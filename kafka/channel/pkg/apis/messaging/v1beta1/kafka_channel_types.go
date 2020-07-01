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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
)

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KafkaChannel is a resource representing a Kafka Channel.
type KafkaChannel struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec defines the desired state of the Channel.
	Spec KafkaChannelSpec `json:"spec,omitempty"`

	// Status represents the current state of the KafkaChannel. This data may be out of
	// date.
	// +optional
	Status KafkaChannelStatus `json:"status,omitempty"`
}

var (
	// Check that this channel can be validated and defaulted.
	_ apis.Validatable = (*KafkaChannel)(nil)
	_ apis.Defaultable = (*KafkaChannel)(nil)

	_ runtime.Object = (*KafkaChannel)(nil)

	// Check that we can create OwnerReferences to an this channel.
	_ kmeta.OwnerRefable = (*KafkaChannel)(nil)

	// Check that the type conforms to the duck Knative Resource shape.
	_ duckv1.KRShaped = (*KafkaChannel)(nil)
)

// KafkaChannelSpec defines the specification for a KafkaChannel.
type KafkaChannelSpec struct {
	// NumPartitions is the number of partitions of a Kafka topic. By default, it is set to 1.
	NumPartitions int32 `json:"numPartitions"`

	// ReplicationFactor is the replication factor of a Kafka topic. By default, it is set to 1.
	ReplicationFactor int16 `json:"replicationFactor"`

	// Channel conforms to Duck type Channelable.
	eventingduck.ChannelableSpec `json:",inline"`
}

// KafkaChannelStatus represents the current state of a KafkaChannel.
type KafkaChannelStatus struct {
	// Channel conforms to Duck type Channelable.
	eventingduck.ChannelableStatus `json:",inline"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KafkaChannelList is a collection of KafkaChannels.
type KafkaChannelList struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KafkaChannel `json:"items"`
}

// GetGroupVersionKind returns GroupVersionKind for KafkaChannels
func (c *KafkaChannel) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("KafkaChannel")
}

// GetStatus retrieves the duck status for this resource. Implements the KRShaped interface.
func (k *KafkaChannel) GetStatus() *duckv1.Status {
	return &k.Status.Status
}
