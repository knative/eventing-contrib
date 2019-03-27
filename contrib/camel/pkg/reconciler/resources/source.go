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

	camelv1alpha1 "github.com/apache/camel-k/pkg/apis/camel/v1alpha1"
	"github.com/knative/eventing-sources/contrib/camel/pkg/apis/sources/v1alpha1"
)

// BuildSourceCode creates the Camel flow code corresponding to the requested source
func BuildSourceCode(source *v1alpha1.CamelSource) (CamelArgumentsSource, error) {
	if source.Spec.Source.Component != nil {
		component := *source.Spec.Source.Component
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

		code, err := flows.Serialize()
		if err != nil {
			return CamelArgumentsSource{}, err
		}

		return CamelArgumentsSource{
			Name:       "source.flow",
			Content:    code,
			Properties: component.Properties,
		}, nil
	}
	return CamelArgumentsSource{}, errors.New("empty source type")
}
