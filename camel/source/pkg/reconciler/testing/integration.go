/*
Copyright 2020 The Knative Authors

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

package testing

import (
	v1 "github.com/apache/camel-k/pkg/apis/camel/v1"
	"knative.dev/eventing-contrib/camel/source/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing-contrib/camel/source/pkg/reconciler/resources"
)

// CamelkIntegrationOption enables further configuration of a Integration.
type CamelkIntegrationOption func(*v1.Integration)

// NewIntegration creates an CamelSource with CamelkIntegrationOption.
func NewIntegration(name, namespace string, src *v1alpha1.CamelSource, ncopt ...CamelkIntegrationOption) *v1.Integration {
	i, _ := resources.MakeIntegration(&resources.CamelArguments{
		Name:      name,
		Namespace: namespace,
		Owner:     src,
		Source:    src.Spec.Source,
		SinkURL:   src.Status.SinkURI,
		//Overrides: nil, ???
	})

	for _, opt := range ncopt {
		opt(i)
	}
	return i
}

func WithIntegrationStatus(status v1.IntegrationStatus) CamelkIntegrationOption {
	return func(nc *v1.Integration) {
		nc.Status = status
	}
}
