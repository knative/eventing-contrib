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
	"sort"

	camelv1alpha1 "github.com/apache/camel-k/pkg/apis/camel/v1alpha1"
	camelknativev1alpha1 "github.com/apache/camel-k/pkg/apis/camel/v1alpha1/knative"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MakeIntegration(args *CamelArguments) (*camelv1alpha1.Integration, error) {
	environment, err := makeCamelEnvironment(args.Sink)
	if err != nil {
		return nil, err
	}

	integration := camelv1alpha1.Integration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: camelv1alpha1.SchemeGroupVersion.String(),
			Kind:       "Integration",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: args.Name + "-",
			Namespace:    args.Namespace,
		},
		Spec: camelv1alpha1.IntegrationSpec{
			Context:            args.Context,
			ServiceAccountName: args.ServiceAccountName,
			Sources: []camelv1alpha1.SourceSpec{
				{
					DataSpec: camelv1alpha1.DataSpec{
						Name:    args.Source.Name,
						Content: args.Source.Content,
					},
				},
			},
			Traits: map[string]camelv1alpha1.IntegrationTraitSpec{
				"knative": {
					Configuration: map[string]string{
						"configuration": environment,
					},
				},
				"images": {
					Configuration: map[string]string{
						"enabled": "true",
					},
				},
			},
		},
	}

	if args.Source.Properties != nil {
		integration.Spec.Configuration = make([]camelv1alpha1.ConfigurationSpec, 0, len(args.Source.Properties))
		// Get keys to have consistent ordering
		keys := make([]string, 0, len(args.Source.Properties))
		for k := range args.Source.Properties {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			conf := camelv1alpha1.ConfigurationSpec{
				Type:  "property",
				Value: k + "=" + args.Source.Properties[k],
			}
			integration.Spec.Configuration = append(integration.Spec.Configuration, conf)
		}
	}

	return &integration, nil
}

func makeCamelEnvironment(sinkURI string) (string, error) {
	env := camelknativev1alpha1.NewCamelEnvironment()
	svc, err := camelknativev1alpha1.BuildCamelServiceDefinition("sink", camelknativev1alpha1.CamelServiceTypeEndpoint, sinkURI)
	if err != nil {
		return "", err
	}
	env.Services = append(env.Services, *svc)
	return env.Serialize()
}
