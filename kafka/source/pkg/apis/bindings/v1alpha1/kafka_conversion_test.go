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

	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/tracker"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	"knative.dev/eventing-contrib/kafka/source/pkg/apis/bindings/v1beta1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
)

func TestKafkaBindingConversionBadType(t *testing.T) {
	good, bad := &KafkaBinding{}, &KafkaBinding{}

	if err := good.ConvertTo(context.Background(), bad); err == nil {
		t.Errorf("ConvertTo() = %#v, wanted error", bad)
	}

	if err := good.ConvertFrom(context.Background(), bad); err == nil {
		t.Errorf("ConvertFrom() = %#v, wanted error", good)
	}
}

// Test v1alpha1 -> v1beta1 -> v1alpha1
func TestKafkaBindingConversionRoundTripV1alpha1(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&v1beta1.KafkaBinding{}}

	tests := []struct {
		name string
		in   *KafkaBinding
	}{{
		name: "min configuration",
		in: &KafkaBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "kafka-binding-name",
				Namespace:  "kafka-binding-ns",
				Generation: 17,
			},
			Spec: KafkaBindingSpec{},
			Status: KafkaBindingStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{},
				},
			},
		},
	}, {
		name: "full configuration",
		in: &KafkaBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "kafka-binding-name",
				Namespace:  "kafka-binding-ns",
				Generation: 17,
			},
			Spec: KafkaBindingSpec{
				BindingSpec: duckv1alpha1.BindingSpec{
					Subject: tracker.Reference{
						APIVersion: "bindingAPIVersion",
						Kind:       "bindingKind",
						Namespace:  "bindingNamespace",
						Name:       "bindingName",
					},
				},
				KafkaAuthSpec: KafkaAuthSpec{
					BootstrapServers: []string{"bootstrap-server-1", "bootstrap-server-2"},
					Net: KafkaNetSpec{
						SASL: KafkaSASLSpec{
							Enable: true,
							User: SecretValueFromSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "sasl-user-secret-local-obj-ref",
									},
									Key:      "sasl-user-secret-key",
									Optional: pointer.BoolPtr(true),
								},
							},
							Password: SecretValueFromSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "sasl-password-secret-local-obj-ref",
									},
									Key:      "sasl-password-secret-key",
									Optional: pointer.BoolPtr(true),
								},
							},
						},
						TLS: KafkaTLSSpec{
							Enable: true,
							Cert: SecretValueFromSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "tls-cert-secret-local-obj-ref",
									},
									Key:      "tls-cert-secret-key",
									Optional: pointer.BoolPtr(true),
								},
							},
							Key: SecretValueFromSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "tls-key-secret-local-obj-ref",
									},
									Key:      "tls-key-secret-key",
									Optional: pointer.BoolPtr(true),
								},
							},
							CACert: SecretValueFromSource{
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
			},
			Status: KafkaBindingStatus{
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

				got := &KafkaBinding{}
				if err := got.ConvertFrom(context.Background(), ver); err != nil {
					t.Errorf("ConvertFrom() = %v", err)
				}

				// Since on the way down, we lose the DeprecatedSourceAndType,
				// convert the in to equivalent out.
				fixed := fixKafkaBindingDeprecated(test.in)

				if diff := cmp.Diff(fixed, got); diff != "" {
					t.Errorf("roundtrip (-want, +got) = %v", diff)
				}
			})
		}
	}
}

// Test v1beta1 -> v1alpha1 -> v1beta1
func TestKafkaBindingConversionRoundTripV1beta1(t *testing.T) {
	// Just one for now, just adding the for loop for ease of future changes.
	versions := []apis.Convertible{&KafkaBinding{}}

	tests := []struct {
		name string
		in   *v1beta1.KafkaBinding
	}{{
		name: "min configuration",
		in: &v1beta1.KafkaBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "kafka-binding-name",
				Namespace:  "kafka-binding-ns",
				Generation: 17,
			},
			Spec: v1beta1.KafkaBindingSpec{},
			Status: v1beta1.KafkaBindingStatus{
				Status: duckv1.Status{
					Conditions: duckv1.Conditions{},
				},
			},
		},
	}, {
		name: "full configuration",
		in: &v1beta1.KafkaBinding{
			ObjectMeta: metav1.ObjectMeta{
				Name:       "kafka-binding-name",
				Namespace:  "kafka-binding-ns",
				Generation: 17,
			},
			Spec: v1beta1.KafkaBindingSpec{
				BindingSpec: duckv1alpha1.BindingSpec{
					Subject: tracker.Reference{
						APIVersion: "bindingAPIVersion",
						Kind:       "bindingKind",
						Namespace:  "bindingNamespace",
						Name:       "bindingName",
					},
				},
				KafkaAuthSpec: v1beta1.KafkaAuthSpec{
					BootstrapServers: []string{"bootstrap-server-1", "bootstrap-server-2"},
					Net: v1beta1.KafkaNetSpec{
						SASL: v1beta1.KafkaSASLSpec{
							Enable: true,
							User: v1beta1.SecretValueFromSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "sasl-user-secret-local-obj-ref",
									},
									Key:      "sasl-user-secret-key",
									Optional: pointer.BoolPtr(true),
								},
							},
							Password: v1beta1.SecretValueFromSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "sasl-password-secret-local-obj-ref",
									},
									Key:      "sasl-password-secret-key",
									Optional: pointer.BoolPtr(true),
								},
							},
						},
						TLS: v1beta1.KafkaTLSSpec{
							Enable: false,
							Cert: v1beta1.SecretValueFromSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "tls-cert-secret-local-obj-ref",
									},
									Key:      "tls-cert-secret-key",
									Optional: pointer.BoolPtr(true),
								},
							},
							Key: v1beta1.SecretValueFromSource{
								SecretKeyRef: &corev1.SecretKeySelector{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "tls-key-secret-local-obj-ref",
									},
									Key:      "tls-key-secret-key",
									Optional: pointer.BoolPtr(true),
								},
							},
							CACert: v1beta1.SecretValueFromSource{
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
			},
			Status: v1beta1.KafkaBindingStatus{
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
				got := &v1beta1.KafkaBinding{}
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
func fixKafkaBindingDeprecated(in *KafkaBinding) *KafkaBinding {
	//in.Spec.ServiceAccountName = ""
	//in.Spec.Resources = KafkaResourceSpec{}
	return in
}
