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
	"testing"

	v1 "knative.dev/eventing/pkg/apis/duck/v1"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"

	"github.com/google/go-cmp/cmp"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"knative.dev/eventing-contrib/kafka/channel/pkg/apis/messaging/v1beta1"
	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestKafkaChannelConversionBadType(t *testing.T) {
	good, bad := &KafkaChannel{}, &KafkaChannel{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

// Test v1alpha1 -> v1beta1 -> v1alpha1
func TestKafkaChannelConversionRoundTripV1alpha1(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&v1beta1.KafkaChannel{}}

	BackoffPolicy := eventingduckv1beta1.BackoffPolicyLinear

	tests := []struct {
		name string
		in   *KafkaChannel
	}{{
		name: "min configuration",
		in: &KafkaChannel{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "kafka-channel-name",
				Namespace:  "kafka-channel-ns",
				Generation: 17,
			},
			Spec: KafkaChannelSpec{
				Subscribable: &eventingduckv1alpha1.Subscribable{},
			},
			Status: KafkaChannelStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{},
				},
				AddressStatus: duckv1alpha1.AddressStatus{
					Address: &duckv1alpha1.Addressable{},
				},
				SubscribableTypeStatus: eventingduckv1alpha1.SubscribableTypeStatus{
					SubscribableStatus: &eventingduckv1alpha1.SubscribableStatus{},
				},
			},
		},
	}, {
		name: "full configuration",
		in: &KafkaChannel{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "kafka-channel-name",
				Namespace:  "kafka-channel-ns",
				Generation: 17,
			},
			Spec: KafkaChannelSpec{
				NumPartitions:     1,
				ReplicationFactor: 2,
				Subscribable: &eventingduckv1alpha1.Subscribable{
					Subscribers: []eventingduckv1alpha1.SubscriberSpec{
						{
							UID:               "subscriber-001-uid",
							Generation:        17,
							SubscriberURI:     apis.HTTP("subscriber-001-uri"),
							ReplyURI:          apis.HTTP("subscriber-001-reply-uri"),
							DeadLetterSinkURI: apis.HTTP("subscriber-001-dls-uri"),
							Delivery: &eventingduckv1beta1.DeliverySpec{
								DeadLetterSink: &duckv1.Destination{
									Ref: &duckv1.KReference{
										Kind:       "subscriber-001-delivery-dls-kind",
										Namespace:  "subscriber-001-delivery-dls-ns",
										Name:       "subscriber-001-delivery-dls-name",
										APIVersion: "subscriber-001-delivery-dls-apiversion",
									},
									URI: apis.HTTP("subscriber-001-dls-uri"),
								},
								Retry:         pointer.Int32Ptr(2),
								BackoffPolicy: &BackoffPolicy,
								BackoffDelay:  pointer.StringPtr("bodelay"),
							},
						},
					},
				},
			},
			Status: KafkaChannelStatus{
				Status: duckv1.Status{
					ObservedGeneration: 1,
					Conditions: duckv1.Conditions{{
						Type:   "Ready",
						Status: "True",
					}},
					Annotations: map[string]string{
						"foo": "bar",
						"hi":  "hello",
					},
				},
				AddressStatus: duckv1alpha1.AddressStatus{
					Address: &duckv1alpha1.Addressable{
						Addressable: duckv1beta1.Addressable{
							URL: apis.HTTP("status-address-url"),
						},
						Hostname: "status-address-url",
					},
				},
				SubscribableTypeStatus: eventingduckv1alpha1.SubscribableTypeStatus{
					SubscribableStatus: &eventingduckv1alpha1.SubscribableStatus{
						Subscribers: []eventingduckv1beta1.SubscriberStatus{
							{
								UID:                "status-subs-uid",
								ObservedGeneration: 43,
								Ready:              "status-subs-ready",
								Message:            "status-subs-msg",
							},
						},
					},
				},
			},
		},
	}}

	for _, test := range tests {
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				if err := test.in.ConvertTo(context.Background(), ver); err != nil {
					t.Errorf("ConvertTo() = %v", err)
				}

				got := &KafkaChannel{}
				if err := got.ConvertFrom(context.Background(), ver); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
				}

				if diff := cmp.Diff(test.in, got); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}

// Test v1beta1 -> v1alpha1 -> v1beta1
func TestKafkaChannelConversionRoundTripV1beta1(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&KafkaChannel{}}

	BackoffPolicyLinear := eventingduckv1.BackoffPolicyLinear

	tests := []struct {
		name string
		in   *v1beta1.KafkaChannel
	}{{
		name: "min configuration",
		in: &v1beta1.KafkaChannel{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "kafka-channel-name",
				Namespace:  "kafka-channel-ns",
				Generation: 17,
			},
			Spec: v1beta1.KafkaChannelSpec{
				ChannelableSpec: eventingduckv1.ChannelableSpec{},
			},
			Status: v1beta1.KafkaChannelStatus{
				ChannelableStatus: eventingduckv1.ChannelableStatus{
					AddressStatus: duckv1.AddressStatus{
						Address: &duckv1.Addressable{},
					},
				},
			},
		},
	}, {
		name: "full configuration",
		in: &v1beta1.KafkaChannel{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "kafka-channel-name",
				Namespace:  "kafka-channel-ns",
				Generation: 17,
			},
			Spec: v1beta1.KafkaChannelSpec{
				NumPartitions:     117,
				ReplicationFactor: 118,
				ChannelableSpec: v1.ChannelableSpec{
					SubscribableSpec: v1.SubscribableSpec{
						Subscribers: []eventingduckv1.SubscriberSpec{
							{
								UID:           "subscriber-uid",
								Generation:    0,
								SubscriberURI: apis.HTTP("subscriber-uri"),
								ReplyURI:      apis.HTTP("subscriber-reply-uri"),
								Delivery: &eventingduckv1.DeliverySpec{
									DeadLetterSink: &duckv1.Destination{
										Ref: &duckv1.KReference{
											Kind:       "subscriber-dls-kind",
											Namespace:  "subscriber-dls-ns",
											Name:       "subscriber-dls-name",
											APIVersion: "subscriber-dls-apiversion",
										},
										URI: apis.HTTP("subscriber-uri"),
									},
									Retry:         pointer.Int32Ptr(10),
									BackoffPolicy: &BackoffPolicyLinear,
									BackoffDelay:  pointer.StringPtr("backoffdelay"),
								},
							},
						},
					},
					// this field doesn't exist in v1alpha1
					//Delivery: &v1.DeliverySpec{
					//	DeadLetterSink: &duckv1.Destination{
					//		Ref: &duckv1.KReference{
					//			Kind:       "delivery-dls-kind",
					//			Namespace:  "delivery-dls-ns",
					//			Name:       "delivery-dls-name",
					//			APIVersion: "delivery-dls-apiversion",
					//		},
					//		URI: apis.HTTP("subscriber-uri"),
					//	},
					//	Retry:         pointer.Int32Ptr(11),
					//	BackoffPolicy: &BackoffPolicyExponential,
					//	BackoffDelay:  pointer.StringPtr("backoffdelay2"),
					//},
				},
			},
			Status: v1beta1.KafkaChannelStatus{
				ChannelableStatus: eventingduckv1.ChannelableStatus{
					Status: duckv1.Status{
						ObservedGeneration: 1,
						Conditions: duckv1.Conditions{{
							Type:   "Ready",
							Status: "True",
						}},
						Annotations: map[string]string{
							"foo": "bar",
							"hi":  "hello",
						},
					},
					AddressStatus: duckv1.AddressStatus{
						Address: &duckv1.Addressable{
							URL: apis.HTTP("status-address-uri"),
						},
					},
					SubscribableStatus: eventingduckv1.SubscribableStatus{
						Subscribers: []eventingduckv1.SubscriberStatus{
							{
								UID:                "status-subs-uid",
								ObservedGeneration: 43,
								Ready:              "status-subs-ready",
								Message:            "status-subs-msg",
							},
						},
					},
					// this field doesn't exist in v1alpha1
					//DeadLetterChannel: &duckv1.KReference{
					//	Kind:       "status-dl-channel-kind",
					//	Namespace:  "status-dl-channel-ns",
					//	Name:       "status-dl-channel-name",
					//	APIVersion: "status-dl-channel-apiversion",
					//},
				},
			},
		},
	}}

	for _, test := range tests {
		for _, version := range versions {
			t.Run(test.name, func(t *testing.T) {
				ver := version
				if err := ver.ConvertFrom(context.Background(), test.in); err != nil {
					t.Errorf("ConvertDown() = %v", err)
				}
				got := &v1beta1.KafkaChannel{}
				if err := ver.ConvertTo(context.Background(), got); err != nil {
					t.Errorf("ConvertUp() = %v", err)
				}

				if diff := cmp.Diff(test.in, got); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}
