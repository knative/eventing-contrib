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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/networking/pkg/apis/networking"
	networkingv1alpha1 "knative.dev/networking/pkg/apis/networking/v1alpha1"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/system"

	sourcesv1alpha1 "knative.dev/eventing-contrib/github/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing-contrib/github/pkg/common"
)

func MakeKIngress(src *sourcesv1alpha1.GitHubSource, host, adapterName string) *networkingv1alpha1.Ingress {
	return &networkingv1alpha1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:      fmt.Sprintf("githubsource-%s", src.Name),
			Namespace: src.Namespace,
			Annotations: map[string]string{
				networking.IngressClassAnnotationKey: "istio.ingress.networking.knative.dev",
			},
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(src),
			},
		},
		Spec: networkingv1alpha1.IngressSpec{
			Rules: []networkingv1alpha1.IngressRule{
				{
					Hosts: []string{host},
					HTTP: &networkingv1alpha1.HTTPIngressRuleValue{
						Paths: []networkingv1alpha1.HTTPIngressPath{
							{
								RewriteHost: fmt.Sprintf("%s.%s.svc.cluster.local", adapterName, system.Namespace()),
								AppendHeaders: map[string]string{
									common.HeaderName:      src.Name,
									common.HeaderNamespace: src.Namespace,
								},
							}},
					},
					Visibility: "ExternalIP",
				},
			},
			Visibility: "ExternalIP",
		},
	}
}
