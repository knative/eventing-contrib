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
package v1alpha1

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// Resource takes an unqualified resource and returns a Group qualified GroupResource
func TestResource(t *testing.T) {
	want := schema.GroupResource{
		Group:    "sources.eventing.knative.dev",
		Resource: "foo",
	}

	got := Resource("foo")

	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected resource (-want, +got) = %v", diff)
	}
}
