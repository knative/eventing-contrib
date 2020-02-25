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

package resources

import (
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"knative.dev/pkg/system"
)

var (
	serviceAccountName = "kafka-ch-dispatcher"
	dispatcherName     = "kafka-ch-dispatcher"
	dispatcherLabels   = map[string]string{
		"messaging.knative.dev/channel": "kafka-channel",
		"messaging.knative.dev/role":    "dispatcher",
	}
)

type DispatcherArgs struct {
	DispatcherScope     string
	DispatcherNamespace string
	Image               string
}

// MakeDispatcher generates the dispatcher deployment for the KafKa channel
func MakeDispatcher(args DispatcherArgs) *v1.Deployment {
	replicas := int32(1)

	return &v1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployments",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      dispatcherName,
			Namespace: args.DispatcherNamespace,
		},
		Spec: v1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: dispatcherLabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: dispatcherLabels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: serviceAccountName,
					Containers: []corev1.Container{
						{
							Name:  "dispatcher",
							Image: args.Image,
							Env:   makeEnv(args),
							Ports: []corev1.ContainerPort{{
								Name:          "metrics",
								ContainerPort: 9090,
							}},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "config-kafka",
									MountPath: "/etc/config-kafka",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "config-kafka",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: "config-kafka",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func makeEnv(args DispatcherArgs) []corev1.EnvVar {
	vars := []corev1.EnvVar{{
		Name:  system.NamespaceEnvKey,
		Value: system.Namespace(),
	}, {
		Name:  "METRICS_DOMAIN",
		Value: "knative.dev/eventing",
	}, {
		Name:  "CONFIG_LOGGING_NAME",
		Value: "config-logging",
	}, {
		Name:  "CONFIG_LEADERELECTION_NAME",
		Value: "config-leader-election-kafka",
	}}

	if args.DispatcherScope == "namespace" {
		vars = append(vars, corev1.EnvVar{
			Name: "NAMESPACE",
			ValueFrom: &corev1.EnvVarSource{
				FieldRef: &corev1.ObjectFieldSelector{
					FieldPath: "metadata.namespace",
				},
			},
		})
	}

	return vars
}
