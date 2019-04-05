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

	sourcesv1alpha1 "github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// MakeContainerSource generates, but does not create, a ContainerSource for the
// given KubernetesEventSource.
func MakeContainerSource(source *sourcesv1alpha1.KubernetesEventSource, receiveAdapterImage string) *sourcesv1alpha1.ContainerSource {
	return &sourcesv1alpha1.ContainerSource{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", source.Name),
			Namespace:    source.Namespace,
		},
		Spec: sourcesv1alpha1.ContainerSourceSpec{
			Image:              receiveAdapterImage,
			Args:               []string{fmt.Sprintf("--namespace=%s", source.Spec.Namespace)},
			ServiceAccountName: source.Spec.ServiceAccountName,
			Sink:               source.Spec.Sink,
		},
	}
}
