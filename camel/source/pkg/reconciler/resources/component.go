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
	"github.com/knative/eventing-contrib/camel/source/pkg/apis/sources/v1alpha1"
)

// BuildComponentIntegrationSpec creates a integration spec corresponding to a component flow
func BuildComponentIntegrationSpec(args *CamelArguments) (camelv1alpha1.IntegrationSpec, error) {
	code, err := buildFlowCode(args.Source)
	if err != nil {
		return camelv1alpha1.IntegrationSpec{}, err
	}

	spec := camelv1alpha1.IntegrationSpec{
		Context:            args.Source.Component.Context,
		ServiceAccountName: args.Source.Component.ServiceAccountName,
		Sources: []camelv1alpha1.SourceSpec{
			{
				DataSpec: camelv1alpha1.DataSpec{
					Name:    "source.flow",
					Content: code,
				},
			},
		},
	}

	// TODO remove when deprecated fields are removed
	if args.DeprecatedServiceAccountName != "" {
		spec.ServiceAccountName = args.DeprecatedServiceAccountName
	}
	if args.DeprecatedIntegrationContext != "" {
		spec.Context = args.DeprecatedIntegrationContext
	}

	if args.Source.Component.Properties != nil {
		spec.Configuration = make([]camelv1alpha1.ConfigurationSpec, 0, len(args.Source.Component.Properties))
		// Get keys to have consistent ordering
		keys := make([]string, 0, len(args.Source.Component.Properties))
		for k := range args.Source.Component.Properties {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			conf := camelv1alpha1.ConfigurationSpec{
				Type:  "property",
				Value: k + "=" + args.Source.Component.Properties[k],
			}
			spec.Configuration = append(spec.Configuration, conf)
		}
	}

	return spec, nil
}

// buildFlowCode creates the Camel flow code corresponding to the requested source
func buildFlowCode(source v1alpha1.CamelSourceOriginSpec) (string, error) {
	component := *source.Component
	flows := camelv1alpha1.Flows{
		{
			Steps: []camelv1alpha1.Step{
				{
					Kind: "endpoint",
					URI:  component.URI,
				},
				{
					Kind: "endpoint",
					URI:  "knative:endpoint/sink",
				},
			},
		},
	}

	return flows.Serialize()
}
