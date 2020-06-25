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
	"knative.dev/eventing-contrib/kafka/source/pkg/apis/bindings/v1beta1"
	bindingsv1beta1 "knative.dev/eventing-contrib/kafka/source/pkg/apis/bindings/v1beta1"
	"knative.dev/pkg/apis"
)

// ConvertTo implements apis.Convertible.
// Converts source (from v1alpha1.KafkaBinding) into v1beta1.KafkaBinding
func (source *KafkaBinding) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *v1beta1.KafkaBinding:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec = v1beta1.KafkaBindingSpec{
			BindingSpec: source.Spec.BindingSpec,
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
		}
		sink.Status.Status = source.Status.Status
		source.Status.Status.ConvertTo(ctx, &sink.Status.Status)
		return nil
	default:
		return fmt.Errorf("Unknown conversion, got: %T", sink)
	}
}

// ConvertFrom implements apis.Convertible.
// Converts obj from v1beta1.KafkaBinding into v1alpha1.KafkaBinding
func (sink *KafkaBinding) ConvertFrom(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *v1beta1.KafkaBinding:
		sink.ObjectMeta = source.ObjectMeta
		sink.Spec = KafkaBindingSpec{
			BindingSpec: source.Spec.BindingSpec,
			KafkaAuthSpec: KafkaAuthSpec{
				BootstrapServers: source.Spec.KafkaAuthSpec.BootstrapServers,
				Net: KafkaNetSpec{
					SASL: KafkaSASLSpec{
						Enable: source.Spec.KafkaAuthSpec.Net.SASL.Enable,
						User: SecretValueFromSource{
							SecretKeyRef: source.Spec.KafkaAuthSpec.Net.SASL.User.SecretKeyRef,
						},
						Password: SecretValueFromSource{
							SecretKeyRef: source.Spec.KafkaAuthSpec.Net.SASL.Password.SecretKeyRef},
					},
					TLS: KafkaTLSSpec{
						Enable: source.Spec.KafkaAuthSpec.Net.TLS.Enable,
						Cert: SecretValueFromSource{
							SecretKeyRef: source.Spec.KafkaAuthSpec.Net.TLS.Cert.SecretKeyRef,
						},
						Key: SecretValueFromSource{
							SecretKeyRef: source.Spec.KafkaAuthSpec.Net.TLS.Key.SecretKeyRef,
						},
						CACert: SecretValueFromSource{
							SecretKeyRef: source.Spec.KafkaAuthSpec.Net.TLS.CACert.SecretKeyRef,
						},
					},
				},
			},
		}
		sink.Status.Status = source.Status.Status
		source.Status.Status.ConvertTo(ctx, &source.Status.Status)
		return nil
	default:
		return fmt.Errorf("Unknown conversion, got: %T", source)
	}
}
