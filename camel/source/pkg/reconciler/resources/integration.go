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
	"fmt"
	"net/url"

	camelv1 "github.com/apache/camel-k/pkg/apis/camel/v1"
	camelknativev1 "github.com/apache/camel-k/pkg/apis/camel/v1/knative"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func MakeIntegration(args *CamelArguments) (*camelv1.Integration, error) {
	if args.Source.Integration == nil && args.Source.Flow == nil {
		return nil, errors.New("empty sources")
	}

	if _, present := args.Overrides["source"]; !present {
		if args.Overrides == nil {
			args.Overrides = make(map[string]string)
		}
		args.Overrides["source"] = fmt.Sprintf("camel-source:%s/%s", args.Namespace, args.Name)
	}

	environment, err := makeCamelEnvironment(args.SinkURL, args.Overrides)
	if err != nil {
		return nil, err
	}

	var spec *camelv1.IntegrationSpec
	if args.Source.Integration != nil {
		spec = args.Source.Integration.DeepCopy()
	} else {
		spec = &camelv1.IntegrationSpec{}
	}

	if args.Source.Flow != nil {
		flows := []map[string]interface{}{*args.Source.Flow}
		flowData, err := MarshalCamelFlows(flows)
		if err != nil {
			return nil, err
		}
		spec.Sources = append(spec.Sources, camelv1.SourceSpec{
			Loader: "knative-source",
			DataSpec: camelv1.DataSpec{
				Name:    "flow.yaml",
				Content: flowData,
			},
		})
	}

	if spec.Traits == nil {
		spec.Traits = make(map[string]camelv1.TraitSpec)
	}
	spec.Traits["knative"] = camelv1.TraitSpec{
		Configuration: map[string]string{
			"configuration": environment,
		},
	}

	integration := camelv1.Integration{
		TypeMeta: metav1.TypeMeta{
			APIVersion: camelv1.SchemeGroupVersion.String(),
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

func makeCamelEnvironment(sinkURIString string, overrides map[string]string) (string, error) {
	sinkURI, err := url.Parse(sinkURIString)
	if err != nil {
		return "", err
	}
	env := camelknativev1.NewCamelEnvironment()
	svc, err := camelknativev1.BuildCamelServiceDefinition(
		"sink",
		camelknativev1.CamelEndpointKindSink,
		camelknativev1.CamelServiceTypeEndpoint,
		*sinkURI,
		"",
		"",
	)
	if err != nil {
		return "", err
	}
	if svc.Metadata == nil {
		svc.Metadata = make(map[string]string)
	}
	for k, v := range overrides {
		svc.Metadata["ce.override.ce-"+k] = v
	}
	env.Services = append(env.Services, svc)
	return env.Serialize()
}
