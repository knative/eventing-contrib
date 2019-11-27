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
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/webhook/resourcesemantics"
)

// +genclient
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RabbitmqSource is the Schema for the rabbitmqsources API.
// +k8s:openapi-gen=true
type RabbitmqSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RabbitmqSourceSpec   `json:"spec,omitempty"`
	Status RabbitmqSourceStatus `json:"status,omitempty"`
}

var _ runtime.Object = (*RabbitmqSource)(nil)
var _ resourcesemantics.GenericCRD = (*RabbitmqSource)(nil)
var _ kmeta.OwnerRefable = (*RabbitmqSource)(nil)
var _ apis.Defaultable = (*RabbitmqSource)(nil)
var _ apis.Validatable = (*RabbitmqSource)(nil)

type RabbitmqChannelConfigSpec struct {
	// Channel Prefetch count
	// +optional
	PrefetchCount int  `json:"prefetch_count,omitempty"`
	// Channel Qos global property
	// +optional
	GlobalQos     bool `json:"global_qos,omitempty"`
}

type RabbitmqSourceExchangeConfigSpec struct {
	// Name of the exchange
	// +optional
	Name        string `json:"name,omitempty"`
	// Type of exchange e.g. direct, topic, headers, fanout
	// +required
	TypeOf      string `json:"type,omitempty"`
	// Exchange is Durable or not
	// +optional
	Durable     bool   `json:"durable,omitempty"`
	// Exchange can be AutoDeleted or not
	// +optional
	AutoDeleted bool   `json:"auto_detected,omitempty"`
	// Declare exchange as internal or not.
	// Exchanges declared as `internal` do not accept accept publishings.
	// +optional
	Internal    bool   `json:"internal,omitempty"`
	// Exchange NoWait property. When set to true,
	// declare without waiting for a confirmation from the server.
	// +optional
	NoWait      bool   `json:"nowait,omitempty"`
}

type RabbitmqSourceQueueConfigSpec struct {
	// Name of the queue to bind to.
	// Queue name may be empty in which case the server will generate a unique name.
	// +optional
	Name             string `json:"name,omitempty"`
	// Routing key of the messages to be received.
	// Multiple routing keys can be specified separated by commas. e.g. key1,key2
	// +optional
	RoutingKey       string `json:"routing_key,omitempty"`
	// Queue is Durable or not
	// +optional
	Durable          bool   `json:"durable,omitempty"`
	// Queue can be AutoDeleted or not
	// +optional
	DeleteWhenUnused bool   `json:"delete_when_unused,omitempty"`
	// Queue is exclusive or not.
	// +optional
	Exclusive        bool   `json:"exclusive,omitempty"`
	// Queue NoWait property. When set to true,
	// the queue will assume to be declared on the server.
	// +optional
	NoWait           bool   `json:"nowait,omitempty"`
}

type RabbitmqSourceSpec struct {
	// Brokers are the Rabbitmq servers the consumer will connect to.
	// +required
	Brokers string `json:"brokers"`
	// Topic topic to consume messages from
	// +required
	Topic   string `json:"topic,omitempty"`
	// User for rabbitmq connection
	// +optional
	User SecretValueFromSource `json:"user,omitempty"`
	// Password for rabbitmq connection
	// +optional
	Password SecretValueFromSource `json:"password,omitempty"`
	// ChannelConfig config for rabbitmq exchange
	// +optional
	ChannelConfig  RabbitmqChannelConfigSpec `json:"channel_config,omitempty"`
	// ExchangeConfig config for rabbitmq exchange
	// +optional
	ExchangeConfig RabbitmqSourceExchangeConfigSpec `json:"exchange_config,omitempty"`
	// QueueConfig config for rabbitmq queues
	// +optional
	QueueConfig RabbitmqSourceQueueConfigSpec `json:"queue_config,omitempty"`
	// Sink is a reference to an object that will resolve to a domain name to use as the sink.
	// +optional
	Sink *duckv1.Destination `json:"sink,omitempty"`
	// ServiceAccoutName is the name of the ServiceAccount that will be used to run the Receive
	// Adapter Deployment.
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
}

// SecretValueFromSource represents the source of a secret value
type SecretValueFromSource struct {
	// The Secret key to select from.
	SecretKeyRef *corev1.SecretKeySelector `json:"secretKeyRef,omitempty"`
}

const (
	RabbitmqEventType = "dev.knative.rabbitmq.event"
)

func RabbitmqEventSource(namespace, rabbitmqSourceName, topic string) string {
	return fmt.Sprintf("/apis/v1/namespaces/%s/rabbitmqsources/%s#%s", namespace, rabbitmqSourceName, topic)
}

type RabbitmqSourceStatus struct {
	// inherits duck/v1alpha1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1.SourceStatus `json:",inline"`
}

func (s *RabbitmqSource) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind("RabbitmqSource")
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// RabbitmqSourceList contains a list of RabbitmqSources.
type RabbitmqSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RabbitmqSource `json:"items"`
}
