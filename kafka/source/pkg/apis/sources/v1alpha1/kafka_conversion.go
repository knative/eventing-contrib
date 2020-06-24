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
	"context"
	"fmt"
	"reflect"

	"knative.dev/eventing-contrib/kafka/source/pkg/apis/bindings/v1alpha1"
	bindingsv1beta1 "knative.dev/eventing-contrib/kafka/source/pkg/apis/bindings/v1beta1"
	"knative.dev/eventing-contrib/kafka/source/pkg/apis/sources/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

// ConvertTo implements apis.Convertible.
// Converts source (from v1alpha1.KafkaSource) into v1beta1.KafkaSource
func (source *KafkaSource) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *v1beta1.KafkaSource:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec = v1beta1.KafkaSourceSpec{
			KafkaAuthSpec: bindingsv1beta1.KafkaAuthSpec{
				BootstrapServers: source.Spec.KafkaAuthSpec.BootstrapServers,
				Net: bindingsv1beta1.KafkaNetSpec{
					SASL: bindingsv1beta1.KafkaSASLSpec{
						Enable: source.Spec.KafkaAuthSpec.Net.SASL.Enable,
						User: bindingsv1beta1.SecretValueFromSource{
							SecretKeyRef: source.Spec.KafkaAuthSpec.Net.SASL.User.SecretKeyRef,
						},
						Password: bindingsv1beta1.SecretValueFromSource{
							SecretKeyRef: source.Spec.KafkaAuthSpec.Net.SASL.Password.SecretKeyRef},
					},
					TLS: bindingsv1beta1.KafkaTLSSpec{
						Enable: source.Spec.KafkaAuthSpec.Net.TLS.Enable,
						Cert: bindingsv1beta1.SecretValueFromSource{
							SecretKeyRef: source.Spec.KafkaAuthSpec.Net.TLS.Cert.SecretKeyRef,
						},
						Key: bindingsv1beta1.SecretValueFromSource{
							SecretKeyRef: source.Spec.KafkaAuthSpec.Net.TLS.Key.SecretKeyRef,
						},
						CACert: bindingsv1beta1.SecretValueFromSource{
							SecretKeyRef: source.Spec.KafkaAuthSpec.Net.TLS.CACert.SecretKeyRef,
						},
					},
				},
			},
			Topics:        source.Spec.Topics,
			ConsumerGroup: source.Spec.ConsumerGroup,
		}
		sink.Status = v1beta1.KafkaSourceStatus{
			duckv1.SourceStatus{
				Status:               source.Status.Status,
				SinkURI:              source.Status.SinkURI,
				CloudEventAttributes: source.Status.CloudEventAttributes,
			},
		}
		// Optionals
		if source.Spec.Sink != nil {
			sink.Spec.Sink = *source.Spec.Sink.DeepCopy()
		}
		if source.Status.SinkURI != nil {
			sink.Status.SinkURI = source.Status.SinkURI.DeepCopy()
		}
		return nil
	default:
		return fmt.Errorf("Unknown conversion, got: %T", sink)
	}
}

// ConvertFrom implements apis.Convertible.
// Converts obj from v1beta1.KafkaSource into v1alpha1.KafkaSource
func (sink *KafkaSource) ConvertFrom(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *v1beta1.KafkaSource:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec = KafkaSourceSpec{
			KafkaAuthSpec: v1alpha1.KafkaAuthSpec{
				BootstrapServers: source.Spec.KafkaAuthSpec.BootstrapServers,
				Net: v1alpha1.KafkaNetSpec{
					SASL: v1alpha1.KafkaSASLSpec{
						Enable: source.Spec.KafkaAuthSpec.Net.SASL.Enable,
						User: v1alpha1.SecretValueFromSource{
							SecretKeyRef: source.Spec.KafkaAuthSpec.Net.SASL.User.SecretKeyRef,
						},
						Password: v1alpha1.SecretValueFromSource{
							SecretKeyRef: source.Spec.KafkaAuthSpec.Net.SASL.Password.SecretKeyRef},
					},
					TLS: v1alpha1.KafkaTLSSpec{
						Enable: source.Spec.KafkaAuthSpec.Net.TLS.Enable,
						Cert: v1alpha1.SecretValueFromSource{
							SecretKeyRef: source.Spec.KafkaAuthSpec.Net.TLS.Cert.SecretKeyRef,
						},
						Key: v1alpha1.SecretValueFromSource{
							SecretKeyRef: source.Spec.KafkaAuthSpec.Net.TLS.Key.SecretKeyRef,
						},
						CACert: v1alpha1.SecretValueFromSource{
							SecretKeyRef: source.Spec.KafkaAuthSpec.Net.TLS.CACert.SecretKeyRef,
						},
					},
				},
			},
			Topics:        source.Spec.Topics,
			ConsumerGroup: source.Spec.ConsumerGroup,
			Sink:          source.Spec.Sink.DeepCopy(),
		}
		if reflect.DeepEqual(*sink.Spec.Sink, duckv1.Destination{}) {
			sink.Spec.Sink = nil
		}
		return nil
	default:
		return fmt.Errorf("Unknown conversion, got: %T", source)
	}
}
