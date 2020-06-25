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
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	apistest "knative.dev/pkg/apis/testing"
)

func TestKafkaBindingDuckTypes(t *testing.T) {
	tests := []struct {
		name string
		t    duck.Implementable
	}{{
		name: "conditions",
		t:    &duckv1.Conditions{},
	}, {
		name: "binding",
		t:    &duckv1alpha1.Binding{},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := duck.VerifyType(&KafkaBinding{}, test.t)
			if err != nil {
				t.Errorf("VerifyType(KafkaBinding, %T) = %v", test.t, err)
			}
		})
	}
}

func TestKafkaBindingGetGroupVersionKind(t *testing.T) {
	r := &KafkaBinding{}
	want := schema.GroupVersionKind{
		Group:   "bindings.knative.dev",
		Version: "v1alpha1",
		Kind:    "KafkaBinding",
	}
	if got := r.GetGroupVersionKind(); got != want {
		t.Errorf("got: %v, want: %v", got, want)
	}
}

func TestKafkaBindingUndo(t *testing.T) {
	url := apis.URL{
		Scheme: "http",
		Host:   "vmware.com",
	}
	secretName := "ssssshhhh-dont-tell"

	tests := []struct {
		name string
		in   *duckv1.WithPod
		want *duckv1.WithPod
	}{{
		name: "nothing to remove",
		in: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
						}},
					},
				},
			},
		},
		want: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
						}},
					},
				},
			},
		},
	}, {
		name: "everything to remove (SASL)",
		in: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "KAFKA_BOOTSTRAP_SERVERS",
								Value: url.String(),
							}, {
								Name:  "KAFKA_NET_SASL_ENABLE",
								Value: "true",
							}, {
								Name: "KAFKA_NET_SASL_USER",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretName,
										},
										Key: corev1.BasicAuthUsernameKey,
									},
								},
							}, {
								Name: "KAFKA_NET_SASL_PASSWORD",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretName,
										},
										Key: corev1.BasicAuthPasswordKey,
									},
								},
							}},
						}},
					},
				},
			},
		},
		want: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env:   []corev1.EnvVar{},
						}},
					},
				},
			},
		},
	}, {
		name: "nothing to add (TLS)",
		in: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "KAFKA_BOOTSTRAP_SERVERS",
								Value: url.String(),
							}, {
								Name:  "KAFKA_NET_TLS_ENABLE",
								Value: "true",
							}, {
								Name: "KAFKA_NET_TLS_CERT",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretName,
										},
										Key: corev1.TLSCertKey,
									},
								},
							}, {
								Name: "KAFKA_NET_TLS_KEY",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretName,
										},
										Key: corev1.TLSPrivateKeyKey,
									},
								},
							}, {
								Name: "KAFKA_NET_TLS_CA_CERT",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretName,
										},
										Key: "ca.crt",
									},
								},
							}},
						}},
					},
				},
			},
		},
		want: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env:   []corev1.EnvVar{},
						}},
					},
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.in
			sb := &KafkaBinding{}
			sb.Undo(context.Background(), got)

			if !cmp.Equal(got, test.want) {
				t.Errorf("Undo (-want, +got): %s", cmp.Diff(test.want, got))
			}
		})
	}
}

func TestKafkaBindingDoSASL(t *testing.T) {
	url := apis.URL{
		Scheme: "http",
		Host:   "vmware.com",
	}
	secretName := "ssssshhhh-dont-tell"
	vsb := &KafkaBinding{
		Spec: KafkaBindingSpec{
			KafkaAuthSpec: KafkaAuthSpec{
				BootstrapServers: []string{
					url.String(),
				},
				Net: KafkaNetSpec{
					SASL: KafkaSASLSpec{
						Enable: true,
						User: SecretValueFromSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: secretName,
								},
								Key: corev1.BasicAuthUsernameKey,
							},
						},
						Password: SecretValueFromSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: secretName,
								},
								Key: corev1.BasicAuthPasswordKey,
							},
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name string
		in   *duckv1.WithPod
		want *duckv1.WithPod
	}{{
		name: "nothing to add (SASL)",
		in: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "KAFKA_BOOTSTRAP_SERVERS",
								Value: url.String(),
							}, {
								Name:  "KAFKA_NET_SASL_ENABLE",
								Value: "true",
							}, {
								Name: "KAFKA_NET_SASL_USER",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretName,
										},
										Key: corev1.BasicAuthUsernameKey,
									},
								},
							}, {
								Name: "KAFKA_NET_SASL_PASSWORD",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretName,
										},
										Key: corev1.BasicAuthPasswordKey,
									},
								},
							}},
						}},
					},
				},
			},
		},
		want: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "KAFKA_BOOTSTRAP_SERVERS",
								Value: url.String(),
							}, {
								Name:  "KAFKA_NET_SASL_ENABLE",
								Value: "true",
							}, {
								Name: "KAFKA_NET_SASL_USER",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretName,
										},
										Key: corev1.BasicAuthUsernameKey,
									},
								},
							}, {
								Name: "KAFKA_NET_SASL_PASSWORD",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretName,
										},
										Key: corev1.BasicAuthPasswordKey,
									},
								},
							}},
						}},
					},
				},
			},
		},
	}, {
		name: "everything to add (SASL)",
		in: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
						}},
					},
				},
			},
		},
		want: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "KAFKA_BOOTSTRAP_SERVERS",
								Value: url.String(),
							}, {
								Name:  "KAFKA_NET_SASL_ENABLE",
								Value: "true",
							}, {
								Name: "KAFKA_NET_SASL_USER",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretName,
										},
										Key: corev1.BasicAuthUsernameKey,
									},
								},
							}, {
								Name: "KAFKA_NET_SASL_PASSWORD",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretName,
										},
										Key: corev1.BasicAuthPasswordKey,
									},
								},
							}},
						}},
					},
				},
			},
		},
	}, {
		name: "everything to fix (SASL)",
		in: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "KAFKA_BOOTSTRAP_SERVERS",
								Value: "bad",
							}, {
								Name:  "KAFKA_NET_SASL_ENABLE",
								Value: "bad",
							}, {
								Name:  "KAFKA_NET_SASL_USER",
								Value: "bad",
							}, {
								Name:  "KAFKA_NET_SASL_PASSWORD",
								Value: "bad",
							}},
						}},
					},
				},
			},
		},
		want: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "KAFKA_BOOTSTRAP_SERVERS",
								Value: url.String(),
							}, {
								Name:  "KAFKA_NET_SASL_ENABLE",
								Value: "true",
							}, {
								Name: "KAFKA_NET_SASL_USER",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretName,
										},
										Key: corev1.BasicAuthUsernameKey,
									},
								},
							}, {
								Name: "KAFKA_NET_SASL_PASSWORD",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretName,
										},
										Key: corev1.BasicAuthPasswordKey,
									},
								},
							}},
						}},
					},
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.in

			ctx := context.Background()

			vsb := vsb.DeepCopy()
			vsb.Do(ctx, got)

			if !cmp.Equal(got, test.want) {
				t.Errorf("Do (-want, +got): %s", cmp.Diff(test.want, got))
			}
		})
	}
}

func TestKafkaBindingDoTLS(t *testing.T) {
	url := apis.URL{
		Scheme: "http",
		Host:   "vmware.com",
	}
	secretName := "ssssshhhh-dont-tell"
	vsb := &KafkaBinding{
		Spec: KafkaBindingSpec{
			KafkaAuthSpec: KafkaAuthSpec{
				BootstrapServers: []string{
					url.String(),
				},
				Net: KafkaNetSpec{
					TLS: KafkaTLSSpec{
						Enable: true,
						Cert: SecretValueFromSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: secretName,
								},
								Key: corev1.TLSCertKey,
							},
						},
						Key: SecretValueFromSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: secretName,
								},
								Key: corev1.TLSPrivateKeyKey,
							},
						},
						CACert: SecretValueFromSource{
							SecretKeyRef: &corev1.SecretKeySelector{
								LocalObjectReference: corev1.LocalObjectReference{
									Name: secretName,
								},
								Key: "ca.crt",
							},
						},
					},
				},
			},
		},
	}

	tests := []struct {
		name string
		in   *duckv1.WithPod
		want *duckv1.WithPod
	}{{
		name: "nothing to add (TLS)",
		in: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "KAFKA_BOOTSTRAP_SERVERS",
								Value: url.String(),
							}, {
								Name:  "KAFKA_NET_TLS_ENABLE",
								Value: "true",
							}, {
								Name: "KAFKA_NET_TLS_CERT",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretName,
										},
										Key: corev1.TLSCertKey,
									},
								},
							}, {
								Name: "KAFKA_NET_TLS_KEY",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretName,
										},
										Key: corev1.TLSPrivateKeyKey,
									},
								},
							}, {
								Name: "KAFKA_NET_TLS_CA_CERT",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretName,
										},
										Key: "ca.crt",
									},
								},
							}},
						}},
					},
				},
			},
		},
		want: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "KAFKA_BOOTSTRAP_SERVERS",
								Value: url.String(),
							}, {
								Name:  "KAFKA_NET_TLS_ENABLE",
								Value: "true",
							}, {
								Name: "KAFKA_NET_TLS_CERT",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretName,
										},
										Key: corev1.TLSCertKey,
									},
								},
							}, {
								Name: "KAFKA_NET_TLS_KEY",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretName,
										},
										Key: corev1.TLSPrivateKeyKey,
									},
								},
							}, {
								Name: "KAFKA_NET_TLS_CA_CERT",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretName,
										},
										Key: "ca.crt",
									},
								},
							}},
						}},
					},
				},
			},
		},
	}, {
		name: "everything to add (TLS)",
		in: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
						}},
					},
				},
			},
		},
		want: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "KAFKA_BOOTSTRAP_SERVERS",
								Value: url.String(),
							}, {
								Name:  "KAFKA_NET_TLS_ENABLE",
								Value: "true",
							}, {
								Name: "KAFKA_NET_TLS_CERT",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretName,
										},
										Key: corev1.TLSCertKey,
									},
								},
							}, {
								Name: "KAFKA_NET_TLS_KEY",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretName,
										},
										Key: corev1.TLSPrivateKeyKey,
									},
								},
							}, {
								Name: "KAFKA_NET_TLS_CA_CERT",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretName,
										},
										Key: "ca.crt",
									},
								},
							}},
						}},
					},
				},
			},
		},
	}, {
		name: "everything to fix (TLS)",
		in: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "KAFKA_BOOTSTRAP_SERVERS",
								Value: "bad",
							}, {
								Name:  "KAFKA_NET_TLS_ENABLE",
								Value: "bad",
							}, {
								Name:  "KAFKA_NET_TLS_CERT",
								Value: "bad",
							}, {
								Name:  "KAFKA_NET_TLS_KEY",
								Value: "bad",
							}},
						}},
					},
				},
			},
		},
		want: &duckv1.WithPod{
			Spec: duckv1.WithPodSpec{
				Template: duckv1.PodSpecable{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{{
							Name:  "blah",
							Image: "busybox",
							Env: []corev1.EnvVar{{
								Name:  "KAFKA_BOOTSTRAP_SERVERS",
								Value: url.String(),
							}, {
								Name:  "KAFKA_NET_TLS_ENABLE",
								Value: "true",
							}, {
								Name: "KAFKA_NET_TLS_CERT",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretName,
										},
										Key: corev1.TLSCertKey,
									},
								},
							}, {
								Name: "KAFKA_NET_TLS_KEY",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretName,
										},
										Key: corev1.TLSPrivateKeyKey,
									},
								},
							}, {
								Name: "KAFKA_NET_TLS_CA_CERT",
								ValueFrom: &corev1.EnvVarSource{
									SecretKeyRef: &corev1.SecretKeySelector{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: secretName,
										},
										Key: "ca.crt",
									},
								},
							}},
						}},
					},
				},
			},
		},
	}}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := test.in

			ctx := context.Background()

			vsb := vsb.DeepCopy()
			vsb.Do(ctx, got)

			if !cmp.Equal(got, test.want) {
				t.Errorf("Do (-want, +got): %s", cmp.Diff(test.want, got))
			}
		})
	}
}

func TestTypicalBindingFlow(t *testing.T) {
	r := &KafkaBindingStatus{}
	r.InitializeConditions()
	apistest.CheckConditionOngoing(r, KafkaBindingConditionReady, t)

	r.MarkBindingUnavailable("Foo", "Bar")
	apistest.CheckConditionFailed(r, KafkaBindingConditionReady, t)

	r.MarkBindingAvailable()
	// After all of that, we're finally ready!
	apistest.CheckConditionSucceeded(r, KafkaBindingConditionReady, t)
}
