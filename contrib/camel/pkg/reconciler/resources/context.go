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
	camelv1alpha1 "github.com/apache/camel-k/pkg/apis/camel/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MakeContext(namespace string, image string) *camelv1alpha1.IntegrationContext {
	ct := camelv1alpha1.IntegrationContext{
		TypeMeta: v1.TypeMeta{
			Kind:       "IntegrationContext",
			APIVersion: "camel.apache.org/v1alpha1",
		},
		ObjectMeta: v1.ObjectMeta{
			GenerateName: "ctx-",
			Namespace:    namespace,
			Labels: map[string]string{
				"app":                           "camel-k",
				"camel.apache.org/context.type": camelv1alpha1.IntegrationContextTypeExternal,
			},
		},
		Spec: camelv1alpha1.IntegrationContextSpec{
			Image:   image,
			Profile: camelv1alpha1.TraitProfileKnative,
		},
	}
	return &ct
}
