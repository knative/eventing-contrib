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
	"knative.dev/pkg/kmeta"

	"knative.dev/eventing-contrib/ceph/pkg/apis/sources/v1alpha1"
)

// ReceiveAdapterArgs are the arguments needed to create a Ceph Source Receive Adapter.
// Every field is required.
type ReceiveAdapterArgs struct {
	Image          string
	Labels         map[string]string
	Source         *v1alpha1.CephSource
	AdditionalEnvs []corev1.EnvVar
}

// MakeReceiveAdapter generates (but does not insert into K8s) the Receive Adapter Deployment for
// Ceph sources.
func MakeReceiveAdapter(args *ReceiveAdapterArgs) *v1.Deployment {
	replicas := int32(1)
	return &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: args.Source.Namespace,
			Name:      kmeta.ChildName(fmt.Sprintf("cephsource-%s-", args.Source.Name), string(args.Source.GetUID())),
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
							Env: append(
								makeEnv(&args.Source.Spec),
								args.AdditionalEnvs...,
							),
						},
					},
				},
			},
		},
	}
}

func makeEnv(spec *v1alpha1.CephSourceSpec) []corev1.EnvVar {
	return []corev1.EnvVar{{
		Name: "NAMESPACE",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.namespace",
			},
		},
	}, {
		Name: "NAME",
		ValueFrom: &corev1.EnvVarSource{
			FieldRef: &corev1.ObjectFieldSelector{
				FieldPath: "metadata.name",
			},
		},
	}, {
		Name:  "METRICS_DOMAIN",
		Value: "knative.dev/eventing",
	}}
}
