package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// KafkaEventSourceStatus defines the observed state of KafkaEventSource
type KafkaEventSourceStatus struct {

	// +optional
	SinkURI string `json:"sinkUri,omitempty"`

	// Nodes are the names of the kafka consumer pods
	Nodes []string `json:"nodes"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KafkaEventSource is the Schema for the kafkaeventsources API
// +k8s:openapi-gen=true
type KafkaEventSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KafkaEventSourceSpec   `json:"spec,omitempty"`
	Status KafkaEventSourceStatus `json:"status,omitempty"`
}

// KafkaEventSourceSpec defines the desired state of KafkaEventSource
type KafkaEventSourceSpec struct {

	// Important: Run "operator-sdk generate k8s" to regenerate code after modifying this file
	Bootstrap string `json:"bootstrap"`

	Topic string `json:"topic"`

	//+optional
	KafkaVersion *string `json:"kafkaVersion,omitempty"`

	//+optional
	ConsumerGroupID *string `json:"consumerGroupID,omitempty"`

	//+optional
	Net KafkaEventSourceNet `json:"net,omitempty"`

	//+optional
	Consumer KafkaEventSourceConsumer `json:"consumer,omitempty"`

	//+optional
	ChannelBufferSize *int64 `json:"channelBufferSize,omitempty"`

	//+optional
	Group KafkaEventSourceGroup `json:"group,omitempty"`

	// Sink is a reference to an object that will resolve to a domain name to use as the sink.
	// +optional
	Sink *corev1.ObjectReference `json:"sink,omitempty"`

	//+optional
	Replicas *int32 `json:"replicas,omitempty"`

	//+optional
	ExternalIPRanges *string `json:"externalIPRanges,omitempty"`
}

//KafkaEventSourceConsumer defines consumer related properties
type KafkaEventSourceConsumer struct {
	// +optional
	MaxWaitTime *int64 `json:"maxWaitTime,omitempty"`
	// +optional
	MaxProcessingTime *int64 `json:"maxProcessingTime,omitempty"`
	// +optional
	Offsets KafkaEventSourceOffsets `json:"offsets,omitempty"`
}

//KafkaEventSourceOffsets offsets information
type KafkaEventSourceOffsets struct {
	//+optional
	CommitInterval *int64 `json:"commitInterval,omitempty"`
	//+optional
	InitialOffset *string `json:"initial,omitempty"`
	//+optional
	Retention *int64 `json:"retention,omitempty"`
	//+optional
	Retry KafkaEventSourceRetry `json:"retry,omitempty"`
}

//KafkaEventSourceRetry retry information
type KafkaEventSourceRetry struct {
	//+optional
	Max *int64 `json:"max,omitempty"`
}

//KafkaEventSourceGroup group information
type KafkaEventSourceGroup struct {
	//+optional
	PartitionStrategy *string `json:"partitionStrategy,omitempty"`
	//+optional
	Session KafkaEventSourceSession `json:"session,omitempty"`
}

//KafkaEventSourceSession session information
type KafkaEventSourceSession struct {
	//+optional
	Timeout *int64 `json:"timeout,omitempty"`
}

// KafkaEventSourceNet defines network related properties
type KafkaEventSourceNet struct {
	MaxOpenRequests *int64 `json:"maxOpenRequests"`
	KeepAlive       *int64 `json:"keepAlive"`

	//+optional
	Sasl KafkaEventSourceSpecSasl `json:"sasl,omitempty"`
}

// KafkaEventSourceSpecSasl defines whether or not and how to use Sasl authentication
type KafkaEventSourceSpecSasl struct {
	Enable    *bool   `json:"enable"`
	Handshake *bool   `json:"handshake"`
	User      *string `json:"user"`
	Password  *string `json:"password"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KafkaEventSourceList contains a list of KafkaEventSource
type KafkaEventSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KafkaEventSource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KafkaEventSource{}, &KafkaEventSourceList{})
}
