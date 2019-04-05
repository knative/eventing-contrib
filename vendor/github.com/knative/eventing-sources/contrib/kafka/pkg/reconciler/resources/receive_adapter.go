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
	"fmt"
	"strconv"

	"github.com/knative/eventing-sources/contrib/kafka/pkg/apis/sources/v1alpha1"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ReceiveAdapterArgs struct {
	Image   string
	Source  *v1alpha1.KafkaSource
	Labels  map[string]string
	SinkURI string
}

func MakeReceiveAdapter(args *ReceiveAdapterArgs) *v1.Deployment {
	replicas := int32(1)
	return &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:    args.Source.Namespace,
			GenerateName: fmt.Sprintf("%s-", args.Source.Name),
			Labels:       args.Labels,
		},
		Spec: v1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: args.Labels,
			},
			Replicas: &replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"sidecar.istio.io/inject": "true",
					},
					Labels: args.Labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: args.Source.Spec.ServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:            "receive-adapter",
							Image:           args.Image,
							ImagePullPolicy: "Always",
							Env: []corev1.EnvVar{
								{
									Name:  "KAFKA_BOOTSTRAP_SERVERS",
									Value: args.Source.Spec.BootstrapServers,
								},
								{
									Name:  "KAFKA_TOPICS",
									Value: args.Source.Spec.Topics,
								},
								{
									Name:  "KAFKA_CONSUMER_GROUP",
									Value: args.Source.Spec.ConsumerGroup,
								},
								{
									Name:  "KAFKA_NET_SASL_ENABLE",
									Value: strconv.FormatBool(args.Source.Spec.Net.SASL.Enable),
								},
								{
									Name:  "KAFKA_NET_SASL_USER",
									Value: args.Source.Spec.Net.SASL.User,
								},
								{
									Name:  "KAFKA_NET_SASL_PASSWORD",
									Value: args.Source.Spec.Net.SASL.Password,
								},
								{
									Name:  "KAFKA_NET_TLS_ENABLE",
									Value: strconv.FormatBool(args.Source.Spec.Net.TLS.Enable),
								},
								{
									Name:  "SINK_URI",
									Value: args.SinkURI,
								},
							},
						},
					},
				},
			},
		},
	}
}
