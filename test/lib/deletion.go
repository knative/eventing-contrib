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

package lib

import (
	"context"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"

	testlib "knative.dev/eventing/test/lib"
)

func DeleteResourceOrFail(ctx context.Context, c *testlib.Client, name string, gvr schema.GroupVersionResource) {
	unstructured := c.Dynamic.Resource(gvr).Namespace(c.Namespace)
	if err := unstructured.Delete(ctx, name, metav1.DeleteOptions{}); err != nil {
		c.T.Fatalf("Failed to delete the resource %q : %v", name, err)
	}
}
