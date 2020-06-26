/*
Copyright 2020 The Knative Authors.

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

	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	pkgduckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"

	"knative.dev/eventing-contrib/kafka/channel/pkg/apis/messaging/v1beta1"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	duckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	duckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	eventingmessaging "knative.dev/eventing/pkg/apis/messaging"
)

// ConvertTo implements apis.Convertible
// Converts source (from v1alpha1.KafkaChannel) into v1beta1.KafkaChannel
func (source *KafkaChannel) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *v1beta1.KafkaChannel:
		sink.ObjectMeta = source.ObjectMeta
		if sink.Annotations == nil {
			sink.Annotations = make(map[string]string)
		}
		sink.Annotations[eventingmessaging.SubscribableDuckVersionAnnotation] = "v1beta1"
		source.Status.ConvertTo(ctx, &sink.Status)
		return source.Spec.ConvertTo(ctx, &sink.Spec)
	default:
		return fmt.Errorf("unknown version, got: %T", sink)
	}
}

// ConvertTo helps implement apis.Convertible
func (source *KafkaChannelSpec) ConvertTo(ctx context.Context, sink *v1beta1.KafkaChannelSpec) error {
	sink.Delivery = source.Delivery
	if source.Subscribable != nil {
		sink.Subscribers = make([]eventingduckv1.SubscriberSpec, len(source.Subscribable.Subscribers))
		for i, s := range source.Subscribable.Subscribers {

			sink.Subscribers[i] = eventingduckv1.SubscriberSpec{
				UID:           s.UID,
				Generation:    s.Generation,
				SubscriberURI: s.SubscriberURI,
				ReplyURI:      s.ReplyURI,
			}
			// If the source has delivery, use it.
			if s.Delivery != nil {
				sink.Subscribers[i].Delivery = s.Delivery
			} else {
				// If however, there's a Deprecated DeadLetterSinkURI, convert that up
				// to DeliverySpec.
				sink.Subscribers[i].Delivery = &eventingduckv1.DeliverySpec{
					DeadLetterSink: &duckv1.Destination{
						URI: s.DeadLetterSinkURI,
					},
				}
			}
		}
	}
	return nil
}

// ConvertTo helps implement apis.Convertible
func (source *KafkaChannelStatus) ConvertTo(ctx context.Context, sink *v1beta1.KafkaChannelStatus) {
	source.Status.ConvertTo(ctx, &sink.Status)
	if source.AddressStatus.Address != nil {
		sink.AddressStatus.Address = &duckv1.Addressable{}
		source.AddressStatus.Address.ConvertTo(ctx, sink.AddressStatus.Address)
	}
	if source.SubscribableTypeStatus.SubscribableStatus != nil &&
		len(source.SubscribableTypeStatus.SubscribableStatus.Subscribers) > 0 {
		sink.SubscribableStatus.Subscribers = make([]eventingduckv1.SubscriberStatus, len(source.SubscribableTypeStatus.SubscribableStatus.Subscribers))
		for i, ss := range source.SubscribableTypeStatus.SubscribableStatus.Subscribers {
			sink.SubscribableStatus.Subscribers[i] = eventingduckv1.SubscriberStatus{
				UID:                ss.UID,
				ObservedGeneration: ss.ObservedGeneration,
				Ready:              ss.Ready,
				Message:            ss.Message,
			}
		}
	}
}

// ConvertFrom implements apis.Convertible.
// Converts obj v1beta1.KafkaChannel into v1alpha1.KafkaChannel
func (sink *KafkaChannel) ConvertFrom(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *v1beta1.KafkaChannel:
		sink.ObjectMeta = source.ObjectMeta
		sink.Status.ConvertFrom(ctx, source.Status)
		sink.Spec.ConvertFrom(ctx, source.Spec)
		if sink.Annotations == nil {
			sink.Annotations = make(map[string]string)
		}
		sink.Annotations[eventingmessaging.SubscribableDuckVersionAnnotation] = "v1alpha1"
		return nil
	default:
		return fmt.Errorf("unknown version, got: %T", source)
	}
}

// ConvertFrom helps implement apis.Convertible
func (sink *KafkaChannelSpec) ConvertFrom(ctx context.Context, source v1beta1.KafkaChannelSpec) {
	sink.Delivery = source.Delivery
	if len(source.Subscribers) > 0 {
		sink.Subscribable = &eventingduck.Subscribable{
			Subscribers: make([]eventingduck.SubscriberSpec, len(source.Subscribers)),
		}

		for i, s := range source.Subscribers {
			sink.Subscribable.Subscribers[i] = eventingduck.SubscriberSpec{
				UID:           s.UID,
				Generation:    s.Generation,
				SubscriberURI: s.SubscriberURI,
				ReplyURI:      s.ReplyURI,
				Delivery:      s.Delivery,
			}
		}
	}
}

// ConvertFrom helps implement apis.Convertible
func (sink *KafkaChannelStatus) ConvertFrom(ctx context.Context, source v1beta1.KafkaChannelStatus) error {
	source.Status.ConvertTo(ctx, &sink.Status)
	if source.AddressStatus.Address != nil {
		sink.AddressStatus.Address = &pkgduckv1alpha1.Addressable{}
		if err := sink.AddressStatus.Address.ConvertFrom(ctx, source.AddressStatus.Address); err != nil {
			return err
		}
	}
	if len(source.SubscribableStatus.Subscribers) > 0 {
		sink.SubscribableTypeStatus.SubscribableStatus = &duckv1alpha1.SubscribableStatus{
			Subscribers: make([]duckv1beta1.SubscriberStatus, len(source.SubscribableStatus.Subscribers)),
		}
		for i, ss := range source.SubscribableStatus.Subscribers {
			sink.SubscribableTypeStatus.SubscribableStatus.Subscribers[i] = duckv1beta1.SubscriberStatus{
				UID:                ss.UID,
				ObservedGeneration: ss.ObservedGeneration,
				Ready:              ss.Ready,
				Message:            ss.Message,
			}
		}
	}
	return nil
}
