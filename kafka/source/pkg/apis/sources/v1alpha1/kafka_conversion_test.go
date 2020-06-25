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

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	bindingsv1alpha1 "knative.dev/eventing-contrib/kafka/source/pkg/apis/bindings/v1alpha1"
	bindingsv1beta1 "knative.dev/eventing-contrib/kafka/source/pkg/apis/bindings/v1beta1"
	"knative.dev/eventing-contrib/kafka/source/pkg/apis/sources/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestKafkaSourceConversionBadType(t *testing.T) {
	good, bad := &KafkaSource{}, &KafkaSource{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

// Test v1alpha1 -> v1beta1 -> v1alpha1
func TestKafkaSourceConversionRoundTripV1alpha1(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&v1beta1.KafkaSource{}}

	tests := []struct {
		name string
		in   *KafkaSource
	}{{
		name: "min configuration",
		in: &KafkaSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "kafka-source-name",
				Namespace:  "kafka-source-ns",
				Generation: 17,
			},
			Spec: KafkaSourceSpec{},
			Status: KafkaSourceStatus{
				SourceStatus: duckv1.SourceStatus{
					Status: duckv1.Status{
						Conditions: duckv1.Conditions{},
					},
				},
			},
		},
	}, {
		name: "full configuration",
		in: &KafkaSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "kafka-source-name",
				Namespace:  "kafka-source-ns",
				Generation: 17,
			},
			Spec: KafkaSourceSpec{
				KafkaAuthSpec: bindingsv1alpha1.KafkaAuthSpec{
					BootstrapServers: []string{"bootstrap-server-1", "bootstrap-server-2"},
					Net: bindingsv1alpha1.KafkaNetSpec{
						SASL: bindingsv1alpha1.KafkaSASLSpec{
							Enable: true,
							User: bindingsv1alpha1.SecretValueFromSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "sasl-user-secret-local-obj-ref",
									},
									Key:      "sasl-user-secret-key",
									Optional: pointer.BoolPtr(true),
								},
							},
							Password: bindingsv1alpha1.SecretValueFromSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "sasl-password-secret-local-obj-ref",
									},
									Key:      "sasl-password-secret-key",
									Optional: pointer.BoolPtr(true),
								},
							},
						},
						TLS: bindingsv1alpha1.KafkaTLSSpec{
							Enable: true,
							Cert: bindingsv1alpha1.SecretValueFromSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "tls-cert-secret-local-obj-ref",
									},
									Key:      "tls-cert-secret-key",
									Optional: pointer.BoolPtr(true),
								},
							},
							Key: bindingsv1alpha1.SecretValueFromSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "tls-key-secret-local-obj-ref",
									},
									Key:      "tls-key-secret-key",
									Optional: pointer.BoolPtr(true),
								},
							},
							CACert: bindingsv1alpha1.SecretValueFromSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "tls-cacert-secret-local-obj-ref",
									},
									Key:      "tls-cacert-secret-key",
									Optional: pointer.BoolPtr(true),
								},
							},
						},
					},
				},
				Topics:        []string{"topic1", "topic2"},
				ConsumerGroup: "consumer-group",
				Sink: &duckv1.Destination{
					Ref: &duckv1.KReference{
						Kind:       "sink-kind",
						Namespace:  "sink-namespace",
						Name:       "sink-name",
						APIVersion: "sink-api-version",
					},
					URI: apis.HTTP("sink-uri"),
				},
				ServiceAccountName: "kafka-sa-name",
				Resources: KafkaResourceSpec{
					Requests: KafkaRequestsSpec{
						ResourceCPU:    "5",
						ResourceMemory: "100m",
					},
					Limits: KafkaLimitsSpec{
						ResourceCPU:    "10",
						ResourceMemory: "200m",
					},
				},
			},
			Status: KafkaSourceStatus{
				SourceStatus: duckv1.SourceStatus{
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
					SinkURI: apis.HTTP("sink-uri"),
					CloudEventAttributes: []duckv1.CloudEventAttributes{
						{
							Type:   "ce-attr-1-type",
							Source: "ce-attr-1-source",
						}, {
							Type:   "ce-attr-2-type",
							Source: "ce-attr-2-source",
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

				got := &KafkaSource{}
				if err := got.ConvertFrom(context.Background(), ver); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
				}

				// Since on the way down, we lose the DeprecatedSourceAndType,
				// convert the in to equivalent out.
				fixed := fixKafkaSourceDeprecated(test.in)

				if diff := cmp.Diff(fixed, got); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}

// Test v1beta1 -> v1alpha1 -> v1beta1
func TestKafkaSourceConversionRoundTripV1beta1(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&KafkaSource{}}

	tests := []struct {
		name string
		in   *v1beta1.KafkaSource
	}{{
		name: "min configuration",
		in: &v1beta1.KafkaSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "kafka-source-name",
				Namespace:  "kafka-source-ns",
				Generation: 17,
			},
			Spec: v1beta1.KafkaSourceSpec{},
			Status: v1beta1.KafkaSourceStatus{
				SourceStatus: duckv1.SourceStatus{
					Status: duckv1.Status{
						Conditions: duckv1.Conditions{},
					},
				},
			},
		},
	}, {
		name: "full configuration",
		in: &v1beta1.KafkaSource{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "kafka-source-name",
				Namespace:  "kafka-source-ns",
				Generation: 17,
			},
			Spec: v1beta1.KafkaSourceSpec{
				SourceSpec: duckv1.SourceSpec{
					Sink: duckv1.Destination{
						Ref: &duckv1.KReference{
							Kind:       "sink-kind",
							Namespace:  "sink-namespace",
							Name:       "sink-name",
							APIVersion: "sink-api-version",
						},
						URI: apis.HTTP("sink-uri"),
					},
					// don't specify the following as v1alpha1 doesn't have that
					//CloudEventOverrides: &duckv1.CloudEventOverrides{
					//	Extensions: map[string]string{
					//		"ext1": "foo",
					//		"ext2": "bar",
					//	},
					//},
				},
				KafkaAuthSpec: bindingsv1beta1.KafkaAuthSpec{
					BootstrapServers: []string{"bootstrap-server-1", "bootstrap-server-2"},
					Net: bindingsv1beta1.KafkaNetSpec{
						SASL: bindingsv1beta1.KafkaSASLSpec{
							Enable: true,
							User: bindingsv1beta1.SecretValueFromSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "sasl-user-secret-local-obj-ref",
									},
									Key:      "sasl-user-secret-key",
									Optional: pointer.BoolPtr(true),
								},
							},
							Password: bindingsv1beta1.SecretValueFromSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "sasl-password-secret-local-obj-ref",
									},
									Key:      "sasl-password-secret-key",
									Optional: pointer.BoolPtr(true),
								},
							},
						},
						TLS: bindingsv1beta1.KafkaTLSSpec{
							Enable: false,
							Cert: bindingsv1beta1.SecretValueFromSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "tls-cert-secret-local-obj-ref",
									},
									Key:      "tls-cert-secret-key",
									Optional: pointer.BoolPtr(true),
								},
							},
							Key: bindingsv1beta1.SecretValueFromSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "tls-key-secret-local-obj-ref",
									},
									Key:      "tls-key-secret-key",
									Optional: pointer.BoolPtr(true),
								},
							},
							CACert: bindingsv1beta1.SecretValueFromSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "tls-cacert-secret-local-obj-ref",
									},
									Key:      "tls-cacert-secret-key",
									Optional: pointer.BoolPtr(true),
								},
							},
						},
					},
				},
				Topics:        []string{"topic1", "topic2"},
				ConsumerGroup: "consumer-group",
			},
			Status: v1beta1.KafkaSourceStatus{
				SourceStatus: duckv1.SourceStatus{
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
					SinkURI: apis.HTTP("sink-uri"),
					CloudEventAttributes: []duckv1.CloudEventAttributes{
						{
							Type:   "ce-attr-1-type",
							Source: "ce-attr-1-source",
						}, {
							Type:   "ce-attr-2-type",
							Source: "ce-attr-2-source",
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
				if err := ver.ConvertFrom(context.Background(), test.in); err != nil {
					t.Errorf("ConvertDown() = %v", err)
				}
				got := &v1beta1.KafkaSource{}
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

// Since v1beta1 to v1alpha1 is lossy but semantically equivalent,
// fix that so diff works.
func fixKafkaSourceDeprecated(in *KafkaSource) *KafkaSource {
	in.Spec.ServiceAccountName = ""
	in.Spec.Resources = KafkaResourceSpec{}
	return in
}
