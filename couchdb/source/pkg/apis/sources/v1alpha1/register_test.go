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
package v1alpha1

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
)

func TestRegisterHelpers(t *testing.T) {
	if got, want := Kind("CouchDbSource"), "CouchDbSource.sources.knative.dev"; got.String() != want {
		t.Errorf("Kind(CouchDbSource) = %v, want %v", got.String(), want)
	}

	if got, want := Resource("CouchDbSource"), "CouchDbSource.sources.knative.dev"; got.String() != want {
		t.Errorf("Resource(CouchDbSource) = %v, want %v", got.String(), want)
	}

	if got, want := SchemeGroupVersion.String(), "sources.knative.dev/v1alpha1"; got != want {
		t.Errorf("SchemeGroupVersion() = %v, want %v", got, want)
	}

	scheme := runtime.NewScheme()
	if err := addKnownTypes(scheme); err != nil {
		t.Errorf("addKnownTypes() = %v", err)
	}
}

// TestKnownTypes makes sure that expected types get added.
func TestKnownTypes(t *testing.T) {
	scheme := runtime.NewScheme()
	addKnownTypes(scheme)
	types := scheme.KnownTypes(SchemeGroupVersion)

	for _, name := range []string{
		"CouchDbSource",
		"CouchDbSourceList",
	} {
		if _, ok := types[name]; !ok {
			t.Errorf("Did not find %q as registered type", name)
		}
	}

}
