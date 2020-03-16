/*
Copyright 2018 The Knative Authors

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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"

	v1 "knative.dev/serving/pkg/apis/serving/v1"

	sourcesv1alpha1 "knative.dev/eventing-contrib/github/pkg/apis/sources/v1alpha1"
)

type ServiceArgs struct {
	ReceiveAdapterImage string
	Source              *sourcesv1alpha1.GitHubSource
}

// MakeService generates, but does not create, a Service for the given
// GitHubSource.
//func MakeService(source *sourcesv1alpha1.GitHubSource, receiveAdapterImage string) *servingv1alpha1.Service {
func MakeService(args *ServiceArgs) *v1.Service {
	labels := map[string]string{
		"receive-adapter": "github",
	}
	sinkURI := args.Source.Status.SinkURI
	env := []corev1.EnvVar{{
		Name: "GITHUB_SECRET_TOKEN",
		ValueFrom: &corev1.EnvVarSource{
			SecretKeyRef: args.Source.Spec.SecretToken.SecretKeyRef,
		},
	}, {
		Name:  "SINK",
		Value: sinkURI,
	}, {
		Name:  "GITHUB_OWNER_REPO",
		Value: args.Source.Spec.OwnerAndRepository,
	}}
	containerArgs := []string{fmt.Sprintf("--sink=%s", sinkURI)}
	return &v1.Service{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", args.Source.Name),
			Namespace:    args.Source.Namespace,
			Labels:       labels,
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(args.Source),
			},
		},
		Spec: v1.ServiceSpec{
			ConfigurationSpec: v1.ConfigurationSpec{
				Template: v1.RevisionTemplateSpec{
					Spec: v1.RevisionSpec{
						PodSpec: corev1.PodSpec{
							Containers: []corev1.Container{{
								Image: args.ReceiveAdapterImage,
								Env:   env,
								Args:  containerArgs,
							}},
						},
					},
				},
			},
		},
	}
}
