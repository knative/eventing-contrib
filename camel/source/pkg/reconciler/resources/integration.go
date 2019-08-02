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
	"errors"
	"net/url"

	camelv1alpha1 "github.com/apache/camel-k/pkg/apis/camel/v1alpha1"
	camelknativev1alpha1 "github.com/apache/camel-k/pkg/apis/camel/v1alpha1/knative"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MakeIntegration(args *CamelArguments) (*camelv1alpha1.Integration, error) {
	if args.Source.DeprecatedComponent != nil && (args.Source.Integration != nil || args.Source.Flow != nil) {
		return nil, errors.New("too many kind of sources defined")
	} else if args.Source.DeprecatedComponent == nil && args.Source.Integration == nil && args.Source.Flow == nil {
		return nil, errors.New("empty sources")
	}

	environment, err := makeCamelEnvironment(args.Sink)
	if err != nil {
		return nil, err
	}

	var spec *camelv1alpha1.IntegrationSpec
	if args.Source.DeprecatedComponent != nil {
		builtSpec, err := BuildComponentIntegrationSpec(args)
		if err != nil {
			return nil, err
		}
		spec = &builtSpec
	} else {
		if args.Source.Integration != nil {
			spec = args.Source.Integration.DeepCopy()
		} else {
			spec = &camelv1alpha1.IntegrationSpec{}
		}

		if args.Source.Flow != nil {
			flow, err := UnmarshalCamelFlow(*args.Source.Flow)
			if err != nil {
				return nil, err
			}
			enhancedFlow := AddSinkToCamelFlow(flow, "sink")
			flows := []map[interface{}]interface{}{enhancedFlow}
			flowData, err := MarshalCamelFlows(flows)
			if err != nil {
				return nil, err
			}
			spec.Sources = append(spec.Sources, camelv1alpha1.SourceSpec{
				Language: camelv1alpha1.LanguageYaml,
				DataSpec: camelv1alpha1.DataSpec{
					Name:    "flow.yaml",
					Content: flowData,
				},
			})
		}
	}

	if spec.Traits == nil {
		spec.Traits = make(map[string]camelv1alpha1.TraitSpec)
	}
	spec.Traits["knative"] = camelv1alpha1.TraitSpec{
		Configuration: map[string]string{
			"configuration": environment,
		},
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
		Spec: *spec,
	}

	return &integration, nil
}

func makeCamelEnvironment(sinkURI string) (string, error) {
	sink, err := url.Parse(sinkURI)
	if err != nil {
		return "", err
	}
	env := camelknativev1alpha1.NewCamelEnvironment()
	svc, err := camelknativev1alpha1.BuildCamelServiceDefinition("sink", camelknativev1alpha1.CamelServiceTypeEndpoint, *sink)
	if err != nil {
		return "", err
	}
	env.Services = append(env.Services, svc)
	return env.Serialize()
}
