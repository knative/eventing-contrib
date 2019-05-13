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

	sourcesv1alpha1 "github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	"github.com/knative/pkg/apis"
	"github.com/knative/pkg/apis/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KafkaSource is the Schema for the kafkasources API.
// +k8s:openapi-gen=true
// +kubebuilder:subresource:status
// +kubebuilder:categories=all,knative,eventing,sources
type KafkaSource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KafkaSourceSpec   `json:"spec,omitempty"`
	Status KafkaSourceStatus `json:"status,omitempty"`
}

// Check that KafkaSource can be validated and can be defaulted.
var _ runtime.Object = (*KafkaSource)(nil)

// Check that KafkaSource will be checked for immutable fields.
var _ apis.Immutable = (*KafkaSource)(nil)

// Check that KafkaSource implements the Conditions duck type.
var _ = duck.VerifyType(&KafkaSource{}, &duckv1alpha1.Conditions{})

type KafkaSourceSASLSpec struct {
	Enable bool `json:"enable,omitempty"`

	// User is the Kubernetes secret containing the SASL username.
	// +optional
	User sourcesv1alpha1.SecretValueFromSource `json:"user,omitempty"`
	// Password is the Kubernetes secret containing the SASL password.
	// +optional
	Password sourcesv1alpha1.SecretValueFromSource `json:"password,omitempty"`
}

type KafkaSourceTLSSpec struct {
	Enable bool `json:"enable,omitempty"`

	// Cert is the Kubernetes secret containing the client certificate.
	// +optional
	Cert sourcesv1alpha1.SecretValueFromSource `json:"cert,omitempty"`
	// Key is the Kubernetes secret containing the client key.
	// +optional
	Key sourcesv1alpha1.SecretValueFromSource `json:"key,omitempty"`
	// CACert is the Kubernetes secret containing the server CA cert.
	// +optional
	CACert sourcesv1alpha1.SecretValueFromSource `json:"caCert,omitempty"`
}

type KafkaSourceNetSpec struct {
	SASL KafkaSourceSASLSpec `json:"sasl,omitempty"`
	TLS  KafkaSourceTLSSpec  `json:"tls,omitempty"`
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
	Sink *corev1.ObjectReference `json:"sink,omitempty"`

	// ServiceAccoutName is the name of the ServiceAccount that will be used to run the Receive
	// Adapter Deployment.
	ServiceAccountName string `json:"serviceAccountName,omitempty"`
}

const (
	// KafkaEventType is the Kafka CloudEvent type.
	KafkaEventType = "dev.knative.kafka.event"
)

// KafkaEventSource returns the Kafka CloudEvent source.
func KafkaEventSource(namespace, kafkaSourceName, topic string) string {
	return fmt.Sprintf("/apis/v1/namespaces/%s/kafkasources/%s#%s", namespace, kafkaSourceName, topic)
}

const (
	// KafkaConditionReady has status True when the KafkaSource is ready to send events.
	KafkaConditionReady = duckv1alpha1.ConditionReady

	// KafkaConditionSinkProvided has status True when the KafkaSource has been configured with a sink target.
	KafkaConditionSinkProvided duckv1alpha1.ConditionType = "SinkProvided"

	// KafkaConditionDeployed has status True when the KafkaSource has had it's receive adapter deployment created.
	KafkaConditionDeployed duckv1alpha1.ConditionType = "Deployed"

	// KafkaConditionEventTypesProvided has status True when the KafkaSource has been configured with event types.
	KafkaConditionEventTypesProvided duckv1alpha1.ConditionType = "EventTypesProvided"
)

var kafkaSourceCondSet = duckv1alpha1.NewLivingConditionSet(
	KafkaConditionSinkProvided,
	KafkaConditionDeployed)

// KafkaSourceStatus defines the observed state of KafkaSource.
type KafkaSourceStatus struct {
	// inherits duck/v1alpha1 Status, which currently provides:
	// * ObservedGeneration - the 'Generation' of the Service that was last processed by the controller.
	// * Conditions - the latest available observations of a resource's current state.
	duckv1alpha1.Status `json:",inline"`

	// SinkURI is the current active sink URI that has been configured for the KafkaSource.
	// +optional
	SinkURI string `json:"sinkUri,omitempty"`
}

// GetCondition returns the condition currently associated with the given type, or nil.
func (s *KafkaSourceStatus) GetCondition(t duckv1alpha1.ConditionType) *duckv1alpha1.Condition {
	return kafkaSourceCondSet.Manage(s).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (s *KafkaSourceStatus) IsReady() bool {
	return kafkaSourceCondSet.Manage(s).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (s *KafkaSourceStatus) InitializeConditions() {
	kafkaSourceCondSet.Manage(s).InitializeConditions()
}

// MarkSink sets the condition that the source has a sink configured.
func (s *KafkaSourceStatus) MarkSink(uri string) {
	s.SinkURI = uri
	if len(uri) > 0 {
		kafkaSourceCondSet.Manage(s).MarkTrue(KafkaConditionSinkProvided)
	} else {
		kafkaSourceCondSet.Manage(s).MarkUnknown(KafkaConditionSinkProvided, "SinkEmpty", "Sink has resolved to empty.%s", "")
	}
}

// MarkNoSink sets the condition that the source does not have a sink configured.
func (s *KafkaSourceStatus) MarkNoSink(reason, messageFormat string, messageA ...interface{}) {
	kafkaSourceCondSet.Manage(s).MarkFalse(KafkaConditionSinkProvided, reason, messageFormat, messageA...)
}

// MarkDeployed sets the condition that the source has been deployed.
func (s *KafkaSourceStatus) MarkDeployed() {
	kafkaSourceCondSet.Manage(s).MarkTrue(KafkaConditionDeployed)
}

// MarkDeploying sets the condition that the source is deploying.
func (s *KafkaSourceStatus) MarkDeploying(reason, messageFormat string, messageA ...interface{}) {
	kafkaSourceCondSet.Manage(s).MarkUnknown(KafkaConditionDeployed, reason, messageFormat, messageA...)
}

// MarkNotDeployed sets the condition that the source has not been deployed.
func (s *KafkaSourceStatus) MarkNotDeployed(reason, messageFormat string, messageA ...interface{}) {
	kafkaSourceCondSet.Manage(s).MarkFalse(KafkaConditionDeployed, reason, messageFormat, messageA...)
}

// MarkEventTypes sets the condition that the source has created its event types.
func (s *KafkaSourceStatus) MarkEventTypes() {
	kafkaSourceCondSet.Manage(s).MarkTrue(KafkaConditionEventTypesProvided)
}

// MarkNoEventTypes sets the condition that the source does not its event types configured.
func (s *KafkaSourceStatus) MarkNoEventTypes(reason, messageFormat string, messageA ...interface{}) {
	kafkaSourceCondSet.Manage(s).MarkFalse(KafkaConditionEventTypesProvided, reason, messageFormat, messageA...)
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// KafkaSourceList contains a list of KafkaSources.
type KafkaSourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KafkaSource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KafkaSource{}, &KafkaSourceList{})
}
