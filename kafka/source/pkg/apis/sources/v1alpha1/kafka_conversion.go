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

	bindingsv1alpha1 "knative.dev/eventing-contrib/kafka/source/pkg/apis/bindings/v1alpha1"
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
		kafkaAuthSpec := bindingsv1beta1.KafkaAuthSpec{}
		if err := source.Spec.KafkaAuthSpec.ConvertTo(ctx, &kafkaAuthSpec); err != nil {
			return err
		}

		sink.ObjectMeta = source.ObjectMeta
		sink.Spec = v1beta1.KafkaSourceSpec{
			KafkaAuthSpec: kafkaAuthSpec,
			Topics:        source.Spec.Topics,
			ConsumerGroup: source.Spec.ConsumerGroup,
		}
		sink.Status.Status = source.Status.Status
		source.Status.Status.ConvertTo(ctx, &sink.Status.Status)
		// Optionals
		if source.Spec.Sink != nil {
			sink.Spec.Sink = *source.Spec.Sink.DeepCopy()
		}
		if source.Status.SinkURI != nil {
			sink.Status.SinkURI = source.Status.SinkURI.DeepCopy()
		}
		if source.Status.CloudEventAttributes != nil {
			sink.Status.CloudEventAttributes = make([]duckv1.CloudEventAttributes, len(source.Status.CloudEventAttributes))
			copy(sink.Status.CloudEventAttributes, source.Status.CloudEventAttributes)
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
		kafkaAuthSpec := bindingsv1alpha1.KafkaAuthSpec{}
		if err := kafkaAuthSpec.ConvertFrom(ctx, &source.Spec.KafkaAuthSpec); err != nil {
			return err
		}

		sink.ObjectMeta = source.ObjectMeta
		sink.Spec = KafkaSourceSpec{
			KafkaAuthSpec: kafkaAuthSpec,
			Topics:        source.Spec.Topics,
			ConsumerGroup: source.Spec.ConsumerGroup,
			Sink:          source.Spec.Sink.DeepCopy(),
		}
		if reflect.DeepEqual(*sink.Spec.Sink, duckv1.Destination{}) {
			sink.Spec.Sink = nil
		}
		sink.Status.Status = source.Status.Status
		source.Status.Status.ConvertTo(ctx, &source.Status.Status)
		// Optionals
		if source.Status.SinkURI != nil {
			sink.Status.SinkURI = source.Status.SinkURI.DeepCopy()
		}
		if source.Status.CloudEventAttributes != nil {
			sink.Status.CloudEventAttributes = make([]duckv1.CloudEventAttributes, len(source.Status.CloudEventAttributes))
			copy(sink.Status.CloudEventAttributes, source.Status.CloudEventAttributes)
		}
		return nil
	default:
		return fmt.Errorf("Unknown conversion, got: %T", source)
	}
}
