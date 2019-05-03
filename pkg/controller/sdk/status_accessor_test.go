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

	"github.com/google/go-cmp/cmp"
	"github.com/knative/eventing-sources/contrib/awssqs/pkg/apis/sources/v1alpha1"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
)

func TestNewReflectedStatusAccessor(t *testing.T) {
	testCases := map[string]struct {
		obj         interface{}
		expectedErr bool
		initial     interface{}
		update      interface{}
	}{
		"obj is a string": {
			obj:         "hello",
			expectedErr: true,
		},
		"status is not present": {
			obj:         struct{}{},
			expectedErr: true,
		},
		"status is not an interface": {
			obj: struct {
				Status int
			}{},
			expectedErr: true,
		},
		"no status set": {
			obj:     &v1alpha1.AwsSqsSource{},
			initial: v1alpha1.AwsSqsSourceStatus{},
		},
		"set status": {
			obj:     &v1alpha1.AwsSqsSource{},
			initial: v1alpha1.AwsSqsSourceStatus{},
			update: v1alpha1.AwsSqsSourceStatus{
				SinkURI: "just testing",
			},
		},
		"set status with conditions": {
			obj:     &v1alpha1.AwsSqsSource{},
			initial: v1alpha1.AwsSqsSourceStatus{},
			update: func() v1alpha1.AwsSqsSourceStatus {
				s := v1alpha1.AwsSqsSourceStatus{
					SinkURI: "updated",
				}
				s.InitializeConditions()
				return s
			}(),
		},
		"replace status": {
			obj: &v1alpha1.AwsSqsSource{
				Status: v1alpha1.AwsSqsSourceStatus{
					SinkURI: "preset",
				},
			},
			initial: v1alpha1.AwsSqsSourceStatus{
				SinkURI: "preset",
				Status: duckv1alpha1.Status{
					Conditions: duckv1alpha1.Conditions(nil),
				},
			},
			update: func() v1alpha1.AwsSqsSourceStatus {
				s := v1alpha1.AwsSqsSourceStatus{
					SinkURI: "updated",
				}
				s.InitializeConditions()
				return s
			}(),
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			sa, err := NewReflectedStatusAccessor(tc.obj)
			if tc.expectedErr != (err != nil) {
				t.Errorf("Unexpected error: %v", err)
			}
			if tc.expectedErr {
				return
			}

			got := sa.GetStatus()
			if diff := cmp.Diff(tc.initial, got); diff != "" {
				t.Errorf("%s: unexpected status before update (-want, +got) = %v", n, diff)
			}
			if tc.update != nil {
				sa.SetStatus(tc.update)
				got := sa.GetStatus()
				if diff := cmp.Diff(tc.update, got); diff != "" {
					t.Errorf("%s: unexpected status after update (-want, +got) = %v", n, diff)
				}
			}
		})
	}
}
