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

	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/utils"
	"knative.dev/pkg/kmeta"

	"knative.dev/eventing-contrib/prometheus/pkg/apis/sources/v1alpha1"
)

// ReceiveAdapterArgs are the arguments needed to create a Prometheus Receive Adapter.
// Every field is required.
type ReceiveAdapterArgs struct {
	EventSource string
	Image       string
	Source      *v1alpha1.PrometheusSource
	Labels      map[string]string
	SinkURI     string
}

// MakeReceiveAdapter generates (but does not insert into K8s) the Receive Adapter Deployment for
// Prometheus sources.
func MakeReceiveAdapter(args *ReceiveAdapterArgs) *v1.Deployment {
	replicas := int32(1)
	ret := &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: args.Source.Namespace,
			Name:      utils.GenerateFixedName(args.Source, fmt.Sprintf("prometheussource-%s", args.Source.Name)),
			Labels:    args.Labels,
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
					Labels: args.Labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: args.Source.Spec.ServiceAccountName,
					Containers: []corev1.Container{
						{
							Name:  "receive-adapter",
							Image: args.Image,
							Env:   makeEnv(args.EventSource, args.SinkURI, &args.Source.Spec),
						},
					},
				},
			},
		},
	}

	if args.Source.Spec.CACertConfigMap != "" {
		ret.Spec.Template.Spec.Containers[0].VolumeMounts = []corev1.VolumeMount{
			{
				Name:      "openshift-service-serving-signer-cabundle",
				MountPath: "/etc/" + args.Source.Spec.CACertConfigMap + "/",
			},
		}
		ret.Spec.Template.Spec.Volumes = []corev1.Volume{
			{
				Name: args.Source.Spec.CACertConfigMap,
				VolumeSource: corev1.VolumeSource{
					ConfigMap: &corev1.ConfigMapVolumeSource{
						LocalObjectReference: corev1.LocalObjectReference{
							Name: args.Source.Spec.CACertConfigMap,
						},
					},
				},
			},
		}
	}
	return ret
}

func makeEnv(eventSource, sinkURI string, spec *v1alpha1.PrometheusSourceSpec) []corev1.EnvVar {
	return []corev1.EnvVar{{
		Name:  "SINK_URI",
		Value: sinkURI,
	}, {
		Name:  "EVENT_SOURCE",
		Value: eventSource,
	}, {
		Name:  "PROMETHEUS_SERVER_URL",
		Value: spec.ServerURL,
	}, {
		Name:  "PROMETHEUS_PROM_QL",
		Value: spec.PromQL,
	}, {
		Name:  "PROMETHEUS_AUTH_TOKEN_FILE",
		Value: spec.AuthTokenFile,
	}, {
		Name:  "PROMETHEUS_CA_CERT_CONFIG_MAP",
		Value: spec.CACertConfigMap,
	}, {
		Name:  "PROMETHEUS_SCHEDULE",
		Value: spec.Schedule,
	}, {
		Name:  "PROMETHEUS_STEP",
		Value: spec.Step,
	}, {
		Name: "NAMESPACE",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.namespace",
			},
		},
	}, {
		Name:  "METRICS_DOMAIN",
		Value: "knative.dev/eventing",
	}, {
		Name:  "K_METRICS_CONFIG",
		Value: "",
	}, {
		Name:  "K_LOGGING_CONFIG",
		Value: "",
	}}
}
