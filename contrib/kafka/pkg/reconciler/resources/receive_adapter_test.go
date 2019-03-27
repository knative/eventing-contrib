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

package resources

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/knative/eventing-sources/contrib/kafka/pkg/apis/sources/v1alpha1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestMakeReceiveAdapter(t *testing.T) {
	src := &v1alpha1.KafkaSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "source-name",
			Namespace: "source-namespace",
		},
		Spec: v1alpha1.KafkaSourceSpec{
			ServiceAccountName: "source-svc-acct",
			Topics:             "topic1,topic2",
			BootstrapServers:   "server1,server2",
			ConsumerGroup:      "group",
			Net: v1alpha1.KafkaSourceNetSpec{
				SASL: v1alpha1.KafkaSourceSASLSpec{
					Enable:   true,
					User:     "user",
					Password: "password",
				},
				TLS: v1alpha1.KafkaSourceTLSSpec{
					Enable: true,
				},
			},
		},
	}

	got := MakeReceiveAdapter(&ReceiveAdapterArgs{
		Image:  "test-image",
		Source: src,
		Labels: map[string]string{
			"test-key1": "test-value1",
			"test-key2": "test-value2",
		},
		SinkURI: "sink-uri",
	})

	one := int32(1)
	want := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    "source-namespace",
			GenerateName: "source-name-",
			Labels: map[string]string{
				"test-key1": "test-value1",
				"test-key2": "test-value2",
			},
		},
		Spec: v1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"test-key1": "test-value1",
					"test-key2": "test-value2",
				},
			},
			Replicas: &one,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"sidecar.istio.io/inject": "true",
					},
					Labels: map[string]string{
						"test-key1": "test-value1",
						"test-key2": "test-value2",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: "source-svc-acct",
					Containers: []corev1.Container{
						{
							Name:            "receive-adapter",
							Image:           "test-image",
							ImagePullPolicy: "Always",
							Env: []corev1.EnvVar{
								{
									Name:  "KAFKA_BOOTSTRAP_SERVERS",
									Value: "server1,server2",
								},
								{
									Name:  "KAFKA_TOPICS",
									Value: "topic1,topic2",
								},
								{
									Name:  "KAFKA_CONSUMER_GROUP",
									Value: "group",
								},
								{
									Name:  "KAFKA_NET_SASL_ENABLE",
									Value: "true",
								},
								{
									Name:  "KAFKA_NET_SASL_USER",
									Value: "user",
								},
								{
									Name:  "KAFKA_NET_SASL_PASSWORD",
									Value: "password",
								},
								{
									Name:  "KAFKA_NET_TLS_ENABLE",
									Value: "true",
								},
								{
									Name:  "SINK_URI",
									Value: "sink-uri",
								},
							},
						},
					},
				},
			},
		},
	}

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected deploy (-want, +got) = %v", diff)
	}
}
