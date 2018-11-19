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

package sinks

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
)

func init() {
	// Add types to scheme
	duckv1alpha1.AddToScheme(scheme.Scheme)
}

func TestGetSinkURI(t *testing.T) {
	testCases := map[string]struct {
		dc        dynamic.Interface
		namespace string
		want      string
		wantErr   error
		ref       *corev1.ObjectReference
	}{
		"happy": {
			dc: getDynamicClient(scheme.Scheme,
				[]runtime.Object{
					// unaddressable resource
					&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "duck.knative.dev/v1alpha1",
							"kind":       "Sink",
							"metadata": map[string]interface{}{
								"namespace": "default",
								"name":      "foo",
							},
							"status": map[string]interface{}{
								"address": map[string]interface{}{
									"hostname": "example.com",
								},
							},
						},
					},
				}),
			namespace: "default",
			ref: &corev1.ObjectReference{
				Kind:       "Sink",
				Name:       "foo",
				APIVersion: "duck.knative.dev/v1alpha1",
			},
			want: "http://example.com/",
		},
		"nil hostname": {
			dc: getDynamicClient(scheme.Scheme,
				[]runtime.Object{
					// unaddressable resource
					&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "duck.knative.dev/v1alpha1",
							"kind":       "Sink",
							"metadata": map[string]interface{}{
								"namespace": "default",
								"name":      "foo",
							},
							"status": map[string]interface{}{
								"address": map[string]interface{}{
									"hostname": nil,
								},
							},
						},
					},
				}),
			namespace: "default",
			ref: &corev1.ObjectReference{
				Kind:       "Sink",
				Name:       "foo",
				APIVersion: "duck.knative.dev/v1alpha1",
			},
			wantErr: fmt.Errorf(`sink contains an empty hostname`),
		},
		"nil sink": {
			dc: getDynamicClient(scheme.Scheme,
				[]runtime.Object{
					// unaddressable resource
					&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "duck.knative.dev/v1alpha1",
							"kind":       "Sink",
							"metadata": map[string]interface{}{
								"namespace": "default",
								"name":      "foo",
							},
							"status": map[string]interface{}{
								"address": map[string]interface{}{
									"hostname": nil,
								},
							},
						},
					},
				}),
			namespace: "default",
			ref:       nil,
			wantErr:   fmt.Errorf(`sink ref is nil`),
		},
		"notSink": {
			dc: getDynamicClient(scheme.Scheme,
				[]runtime.Object{
					// unaddressable resource
					&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "duck.knative.dev/v1alpha1",
							"kind":       "KResource",
							"metadata": map[string]interface{}{
								"namespace": "default",
								"name":      "foo",
							},
						},
					},
				}),
			namespace: "default",
			ref: &corev1.ObjectReference{
				Kind:       "KResource",
				Name:       "foo",
				APIVersion: "duck.knative.dev/v1alpha1",
			},
			wantErr: fmt.Errorf(`sink does not contain address`),
		},
		"notFound": {
			dc: getDynamicClient(scheme.Scheme,
				nil),
			namespace: "default",
			ref: &corev1.ObjectReference{
				Kind:       "KResource",
				Name:       "foo",
				APIVersion: "duck.knative.dev/v1alpha1",
			},
			wantErr: fmt.Errorf(`kresources.duck.knative.dev "foo" not found`),
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			uri, gotErr := GetSinkURI(tc.dc, tc.ref, tc.namespace)
			if gotErr != nil {
				if tc.wantErr != nil {
					if diff := cmp.Diff(tc.wantErr.Error(), gotErr.Error()); diff != "" {
						t.Errorf("%s: unexpected error (-want, +got) = %v", n, diff)
					}
				} else {
					t.Errorf("%s: unexpected error %v", n, gotErr.Error())
				}
			}
			if gotErr == nil {
				got := uri
				if diff := cmp.Diff(tc.want, got); diff != "" {
					t.Errorf("%s: unexpected object (-want, +got) = %v", n, diff)
				}
			}
		})
	}
}

func TestFetchObjectReference(t *testing.T) {
	testCases := map[string]struct {
		dc        dynamic.Interface
		namespace string
		want      *unstructured.Unstructured
		wantErr   error
		ref       *corev1.ObjectReference
	}{
		"happy": {
			dc: getDynamicClient(scheme.Scheme,
				[]runtime.Object{
					// unaddressable resource
					&unstructured.Unstructured{
						Object: map[string]interface{}{
							"apiVersion": "duck.knative.dev/v1alpha1",
							"kind":       "KResource",
							"metadata": map[string]interface{}{
								"namespace": "default",
								"name":      "foo",
							},
						},
					},
				}),
			ref: &corev1.ObjectReference{
				Kind:       "KResource",
				Name:       "foo",
				Namespace:  "default",
				APIVersion: "duck.knative.dev/v1alpha1",
			},
			namespace: "default",
			want: &unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": "duck.knative.dev/v1alpha1",
					"kind":       "KResource",
					"metadata": map[string]interface{}{
						"namespace": "default",
						"name":      "foo",
					},
				},
			},
			wantErr: error(nil),
		},
		"dynamicClientError": {
			dc: getErrorClient(),
			ref: &corev1.ObjectReference{
				Kind:       "Kind",
				Name:       "Name",
				APIVersion: "api/version",
			},
			namespace: "default",
			wantErr:   fmt.Errorf("failed to create dynamic client resource"),
		},
		"errorNotFound": {
			dc: getDynamicClient(nil, nil),
			ref: &corev1.ObjectReference{
				Kind:       "Kind",
				Name:       "Name",
				APIVersion: "api/version",
			},
			namespace: "default",
			wantErr:   fmt.Errorf(`kinds.api "Name" not found`),
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			obj, gotErr := fetchObjectReference(tc.dc, tc.ref, tc.namespace)

			if tc.wantErr != gotErr {
				if diff := cmp.Diff(tc.wantErr.Error(), gotErr.Error()); diff != "" {
					t.Errorf("unexpected error (-want, +got) = %v", diff)
				}
			}

			var want []byte
			var got []byte

			if gotErr == nil && obj != nil {
				got, _ = obj.MarshalJSON()
			}
			if tc.want != nil {
				want, _ = tc.want.MarshalJSON()
			}

			if diff := cmp.Diff(string(want), string(got)); diff != "" {
				t.Errorf("unexpected object (-want, +got) = %v", diff)
			}
		})
	}

}

func TestCreateResourceInterface(t *testing.T) {
	testCases := map[string]struct {
		dc        dynamic.Interface
		namespace string
		wantNil   bool
		wantErr   error
		ref       *corev1.ObjectReference
	}{
		"happy": {
			dc: getDynamicClient(nil, nil),
			ref: &corev1.ObjectReference{
				Kind:       "Kind",
				Name:       "Name",
				APIVersion: "api/version",
			},
			namespace: "default",
			wantErr:   error(nil),
			wantNil:   false,
		},
		"error": {
			dc: getErrorClient(),
			ref: &corev1.ObjectReference{
				Kind:       "Kind",
				Name:       "Name",
				APIVersion: "api/version",
			},
			namespace: "default",
			wantErr:   fmt.Errorf("failed to create dynamic client resource"),
			wantNil:   true,
		},
	}
	for n, tc := range testCases {
		t.Run(n, func(t *testing.T) {
			got, gotErr := createResourceInterface(tc.dc, tc.ref, tc.namespace)

			if tc.wantErr != gotErr {
				if diff := cmp.Diff(tc.wantErr.Error(), gotErr.Error()); diff != "" {
					t.Errorf("unexpected error (-want, +got) = %v", diff)
				}
			}

			if (got != nil) == tc.wantNil {
				t.Errorf("%s: unexpected interface wanted nil %v, got %v", n, tc.wantNil, got == nil)
			}
		})
	}
}

// GetDynamicClient returns the mockDynamicClient to use for this test case.
func getDynamicClient(scheme *runtime.Scheme, objs []runtime.Object) dynamic.Interface {
	if scheme == nil {
		return dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), objs...)
	}
	return dynamicfake.NewSimpleDynamicClient(scheme, objs...)
}

func getErrorClient() dynamic.Interface {
	return &errorClient{}
}

type errorClient struct{}

var _ dynamic.Interface = &errorClient{}

func (c *errorClient) Resource(resource schema.GroupVersionResource) dynamic.NamespaceableResourceInterface {
	return nil
}
