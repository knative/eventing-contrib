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

	"knative.dev/eventing-contrib/kafka/channel/pkg/apis/messaging/v1beta1"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
)

// ConvertTo implements apis.Convertible.
// Converts channel (from v1alpha1.KafkaChannel) into v1beta1.KafkaChannel
func (source *KafkaChannel) ConvertTo(ctx context.Context, obj apis.Convertible) error {
	switch sink := obj.(type) {
	case *v1beta1.KafkaChannel:
		statusAddressable := &duckv1.Addressable{
			URL: nil,
		}
		if source.Status.AddressStatus.Address != nil {
			if err := source.Status.AddressStatus.Address.ConvertTo(ctx, statusAddressable); err != nil {
				return err
			}
		}

		subscribableStatus := eventingduckv1.SubscribableStatus{
			Subscribers: nil,
		}
		if source.Status.SubscribableStatus != nil && len(source.Status.SubscribableStatus.Subscribers) > 0 {
			subscribableStatus.Subscribers = make([]eventingduckv1.SubscriberStatus, len(source.Status.SubscribableStatus.Subscribers))
			for i, ss := range source.Status.SubscribableStatus.Subscribers {
				subscribableStatus.Subscribers[i] = eventingduckv1.SubscriberStatus{
					UID:                ss.UID,
					ObservedGeneration: ss.ObservedGeneration,
					Ready:              ss.Ready,
					Message:            ss.Message,
				}
			}
		}

		subscribableSpec := eventingduckv1.SubscribableSpec{
			Subscribers: nil,
		}
		if source.Spec.Subscribable != nil && len(source.Spec.Subscribable.Subscribers) > 0 {
			subscribableSpec.Subscribers = make([]eventingduckv1.SubscriberSpec, len(source.Spec.Subscribable.Subscribers))
			for i, ss := range source.Spec.Subscribable.Subscribers {
				var subscriberURI *apis.URL
				if ss.SubscriberURI != nil {
					subscriberURI = ss.SubscriberURI.DeepCopy()
				}

				subscribableSpec.Subscribers[i] = eventingduckv1.SubscriberSpec{
					UID:           ss.UID,
					Generation:    ss.Generation,
					SubscriberURI: subscriberURI,
					ReplyURI:      ss.ReplyURI,
				}
				if ss.Delivery != nil {
					delivery := &eventingduckv1.DeliverySpec{}
					if err := ss.Delivery.ConvertTo(ctx, delivery); err != nil {
						return err
					}
					subscribableSpec.Subscribers[i].Delivery = delivery
				}
			}
		}

		sink.ObjectMeta = source.ObjectMeta
		sink.Spec = v1beta1.KafkaChannelSpec{
			NumPartitions:     source.Spec.NumPartitions,
			ReplicationFactor: source.Spec.ReplicationFactor,
			ChannelableSpec: eventingduckv1.ChannelableSpec{
				SubscribableSpec: subscribableSpec,
				// no delivery in v1alpha1
				Delivery: nil,
			},
		}
		sink.Status = v1beta1.KafkaChannelStatus{
			ChannelableStatus: eventingduckv1.ChannelableStatus{
				Status: source.Status.Status,
				AddressStatus: duckv1.AddressStatus{
					Address: statusAddressable,
				},
				SubscribableStatus: subscribableStatus,
				// no DeadLetterChannel in v1alpha1
				DeadLetterChannel: nil,
			},
		}

		return nil
	default:
		return fmt.Errorf("Unknown conversion, got: %T", sink)
	}
}

// ConvertFrom implements apis.Convertible.
// Converts obj from v1beta1.KafkaChannel into v1alpha1.KafkaChannel
func (sink *KafkaChannel) ConvertFrom(ctx context.Context, obj apis.Convertible) error {
	switch source := obj.(type) {
	case *v1beta1.KafkaChannel:
		statusAddressable := &duckv1alpha1.Addressable{}
		if source.Status.AddressStatus.Address != nil {
			if err := statusAddressable.ConvertFrom(ctx, source.Status.AddressStatus.Address); err != nil {
				return err
			}
		}

		subscribableStatus := &eventingduckv1alpha1.SubscribableStatus{
			Subscribers: nil,
		}
		if len(source.Status.SubscribableStatus.Subscribers) > 0 {
			subscribableStatus.Subscribers = make([]eventingduckv1beta1.SubscriberStatus, len(source.Status.SubscribableStatus.Subscribers))
			for i, ss := range source.Status.SubscribableStatus.Subscribers {
				subscribableStatus.Subscribers[i] = eventingduckv1beta1.SubscriberStatus{
					UID:                ss.UID,
					ObservedGeneration: ss.ObservedGeneration,
					Ready:              ss.Ready,
					Message:            ss.Message,
				}
			}
		}

		subscribableSpec := eventingduckv1alpha1.Subscribable{
			Subscribers: nil,
		}
		if len(source.Spec.SubscribableSpec.Subscribers) > 0 {
			subscribableSpec.Subscribers = make([]eventingduckv1alpha1.SubscriberSpec, len(source.Spec.SubscribableSpec.Subscribers))
			for i, ss := range source.Spec.SubscribableSpec.Subscribers {
				var deadLetterSinkURI *apis.URL
				if ss.Delivery != nil && ss.Delivery.DeadLetterSink != nil && ss.Delivery.DeadLetterSink.URI != nil {
					deadLetterSinkURI = ss.Delivery.DeadLetterSink.URI.DeepCopy()
				}

				subscribableSpec.Subscribers[i] = eventingduckv1alpha1.SubscriberSpec{
					UID:               ss.UID,
					Generation:        ss.Generation,
					SubscriberURI:     ss.SubscriberURI.DeepCopy(),
					ReplyURI:          ss.ReplyURI,
					DeadLetterSinkURI: deadLetterSinkURI,
				}

				if ss.Delivery != nil {
					delivery := &eventingduckv1beta1.DeliverySpec{}
					if err := delivery.ConvertFrom(ctx, ss.Delivery); err != nil {
						return err
					}
					subscribableSpec.Subscribers[i].Delivery = delivery
				}
			}
		}

		sink.ObjectMeta = source.ObjectMeta
		sink.Spec = KafkaChannelSpec{
			NumPartitions:     source.Spec.NumPartitions,
			ReplicationFactor: source.Spec.ReplicationFactor,
			Subscribable:      &subscribableSpec,
		}
		sink.Status = KafkaChannelStatus{
			Status: source.Status.Status,
			AddressStatus: duckv1alpha1.AddressStatus{
				Address: statusAddressable,
			},
			SubscribableTypeStatus: eventingduckv1alpha1.SubscribableTypeStatus{
				SubscribableStatus: subscribableStatus,
			},
		}

		return nil
	default:
		return fmt.Errorf("Unknown conversion, got: %T", source)
	}
}
