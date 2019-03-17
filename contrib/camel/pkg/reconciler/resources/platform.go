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

const (
	// CamelVersion is the version of the Apache Camel core library to use
	CamelVersion = "2.23.1"

	// IntegrationPlatformName is the standard name of the Camel K IntegrationPlatform resource
	IntegrationPlatformName = "camel-k"
)

func MakePlatform(namespace string) *camelv1alpha1.IntegrationPlatform {
	pl := camelv1alpha1.IntegrationPlatform{
		TypeMeta: v1.TypeMeta{
			Kind:       "IntegrationPlatform",
			APIVersion: "camel.apache.org/v1alpha1",
		},
		ObjectMeta: v1.ObjectMeta{
			Name:      IntegrationPlatformName,
			Namespace: namespace,
			Labels: map[string]string{
				"app": IntegrationPlatformName,
			},
		},
		Spec: camelv1alpha1.IntegrationPlatformSpec{
			Build: camelv1alpha1.IntegrationPlatformBuildSpec{
				CamelVersion: CamelVersion,
			},
			Profile: camelv1alpha1.TraitProfileKnative,
			Resources: camelv1alpha1.IntegrationPlatformResourcesSpec{
				Contexts: []string{"none"},
			},
		},
	}
	return &pl
}
