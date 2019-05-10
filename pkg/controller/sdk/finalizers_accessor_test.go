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

package sdk

import (
	"testing"

	"github.com/knative/eventing-sources/contrib/awssqs/pkg/apis/sources/v1alpha1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

func TestNewReflectedFinalizersAccessor(t *testing.T) {
	testCases := map[string]struct {
		obj           interface{}
		expectedErr   bool
		finalizers    sets.String
		setFinalizers sets.String
	}{
		"obj is a string": {
			obj:         "hello",
			expectedErr: true,
		},
		"finalizers is not present": {
			obj:         struct{}{},
			expectedErr: true,
		},
		"finalizers is not a slice": {
			obj: struct {
				Finalizers int
			}{},
			expectedErr: true,
		},
		"no finalizers set": {
			obj:        &v1alpha1.AwsSqsSource{},
			finalizers: sets.NewString(),
		},
		"set finalizers": {
			obj:           &v1alpha1.AwsSqsSource{},
			finalizers:    sets.NewString(),
			setFinalizers: sets.NewString("newFinalizer"),
		},
		"replace finalizers": {
			obj: &v1alpha1.AwsSqsSource{
				ObjectMeta: v1.ObjectMeta{
					Finalizers: []string{
						"oldFinalizer",
					},
				},
			},
			finalizers:    sets.NewString("oldFinalizer"),
			setFinalizers: sets.NewString("newFinalizer"),
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			fa, err := NewReflectedFinalizersAccessor(tc.obj)
			if tc.expectedErr != (err != nil) {
				t.Errorf("Unexpected error: %v", err)
			}
			if tc.expectedErr {
				return
			}

			if !tc.finalizers.Equal(fa.GetFinalizers()) {
				t.Errorf("Finalizers aren't equal. Expected '%v'. Actual '%v'", tc.finalizers, fa.GetFinalizers())
			}
			if tc.setFinalizers != nil {
				fa.SetFinalizers(tc.setFinalizers)
				if !tc.setFinalizers.Equal(fa.GetFinalizers()) {
					t.Errorf("Finalizers aren't equal. Expected '%v'. Actual '%v'", tc.setFinalizers, fa.GetFinalizers())
				}
			}
		})
	}
}
