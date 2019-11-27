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
	v1alpha1 "knative.dev/eventing-contrib/rabbitmq/source/pkg/apis/sources/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/api/apps/v1"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/pkg/kmeta"
	"strconv"
)

type ReceiveAdapterArgs struct {
	Image   string
	Source  *v1alpha1.RabbitmqSource
	Labels  map[string]string
	SinkURI string
	MetricsConfig string
	LoggingConfig string
}

func MakeReceiveAdapter(args *ReceiveAdapterArgs) *v1.Deployment {
	replicas := int32(1)

	env := []corev1.EnvVar{
		{
			Name:  "RABBITMQ_BROKERS",
			Value: args.Source.Spec.Brokers,
		},
		{
			Name:  "RABBITMQ_TOPIC",
			Value: args.Source.Spec.Topic,
		},
		{
			Name: "RABBITMQ_USER",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: args.Source.Spec.User.SecretKeyRef,
			},
		},
		{
			Name: "RABBITMQ_PASSWORD",
			ValueFrom: &corev1.EnvVarSource{
				SecretKeyRef: args.Source.Spec.Password.SecretKeyRef,
			},
		},
		{
			Name:  "RABBITMQ_ROUTING_KEY",
			Value: args.Source.Spec.QueueConfig.RoutingKey,
		},
		{
			Name:  "RABBITMQ_CHANNEL_CONFIG_PREFETCH_COUNT",
			Value: strconv.Itoa(args.Source.Spec.ChannelConfig.PrefetchCount),
		},
		{
			Name:  "RABBITMQ_CHANNEL_CONFIG_QOS_GLOBAL",
			Value: strconv.FormatBool(args.Source.Spec.ChannelConfig.GlobalQos),
		},
		{
			Name:  "RABBITMQ_EXCHANGE_CONFIG_NAME",
			Value: args.Source.Spec.ExchangeConfig.Name,
		},
		{
			Name:  "RABBITMQ_EXCHANGE_CONFIG_TYPE",
			Value: args.Source.Spec.ExchangeConfig.TypeOf,
		},
		{
			Name: "RABBITMQ_EXCHANGE_CONFIG_DURABLE",
			Value: strconv.FormatBool(args.Source.Spec.ExchangeConfig.Durable),
		},
		{
			Name: "RABBITMQ_EXCHANGE_CONFIG_AUTO_DELETED",
			Value: strconv.FormatBool(args.Source.Spec.ExchangeConfig.AutoDeleted),
		},
		{
			Name: "RABBITMQ_EXCHANGE_CONFIG_INTERNAL",
			Value: strconv.FormatBool(args.Source.Spec.ExchangeConfig.Internal),
		},
		{
			Name: "RABBITMQ_EXCHANGE_CONFIG_NOWAIT",
			Value: strconv.FormatBool(args.Source.Spec.ExchangeConfig.NoWait),
		},
		{
			Name: "RABBITMQ_QUEUE_CONFIG_NAME",
			Value: args.Source.Spec.QueueConfig.Name,
		},
		{
			Name: "RABBITMQ_QUEUE_CONFIG_DURABLE",
			Value: strconv.FormatBool(args.Source.Spec.QueueConfig.Durable),
		},
		{
			Name: "RABBITMQ_QUEUE_CONFIG_AUTO_DELETED",
			Value: strconv.FormatBool(args.Source.Spec.QueueConfig.DeleteWhenUnused),
		},
		{
			Name: "RABBITMQ_QUEUE_CONFIG_EXCLUSIVE",
			Value: strconv.FormatBool(args.Source.Spec.QueueConfig.Exclusive),
		},
		{
			Name: "RABBITMQ_QUEUE_CONFIG_NOWAIT",
			Value: strconv.FormatBool(args.Source.Spec.QueueConfig.NoWait),
		},
		{
			Name: "SINK_URI",
			Value: args.SinkURI,
		},
		{
			Name:  "K_SINK",
			Value: args.SinkURI,
		},
		{
			Name:  "NAME",
			Value: args.Source.Name,
		},
		{
			Name:  "NAMESPACE",
			Value: args.Source.Namespace,
		},
		{
			Name:  "K_LOGGING_CONFIG",
			Value: args.LoggingConfig,
		},
		{
			Name:  "K_METRICS_CONFIG",
			Value: args.MetricsConfig,
		},
	}

	return &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:         utils.GenerateFixedName(args.Source, fmt.Sprintf("rabbitmqsource-%s", args.Source.Name)),
			Namespace:    args.Source.Namespace,
			GenerateName: fmt.Sprintf("%s-", args.Source.Name),
			Labels:       args.Labels,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(args.Source),
			},
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
							Name:  "receive-adapter",
							Image: args.Image,
							ImagePullPolicy: "IfNotPresent",
							Env: env,
						},
					},
				},
			},
		},
	}
}