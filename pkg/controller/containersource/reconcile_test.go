/*
Copyright 2018 The Knative Authors

Licensed under the Apache License, Veroute.on 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package containersource

import (
	"fmt"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	//"fmt"
	"testing"

	sourcesv1alpha1 "github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	controllertesting "github.com/knative/eventing-sources/pkg/controller/testing"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	//"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	//"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	targetURI = "http://sinkable.sink.svc.cluster.local/"
)

const (
	image               = "github.com/knative/test/image"
	containerSourceName = "testcontainersource"
	testNS              = "testnamespace"

	sinkableDNS = "sinkable.sink.svc.cluster.local"

	sinkableName       = "testsink"
	sinkableKind       = "Sink"
	sinkableAPIVersion = "duck.knative.dev/v1alpha1"

	unsinkableName       = "testunsinkable"
	unsinkableKind       = "KResource"
	unsinkableAPIVersion = "duck.knative.dev/v1alpha1"

	sinkServiceName       = "testsinkservice"
	sinkServiceKind       = "Service"
	sinkServiceAPIVersion = "v1"
)

// Adds the list of known types to Scheme.
func duckAddKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(
		duckv1alpha1.SchemeGroupVersion,
		&duckv1alpha1.SinkList{},
	)
	metav1.AddToGroupVersion(scheme, duckv1alpha1.SchemeGroupVersion)
	return nil
}

func init() {
	// Add types to scheme
	sourcesv1alpha1.SchemeBuilder.AddToScheme(scheme.Scheme)
	duckv1alpha1.AddToScheme(scheme.Scheme)
	duckAddKnownTypes(scheme.Scheme)
}

var testCases = []controllertesting.TestCase{
	{
		Name:         "non existent key",
		Reconciles:   &sourcesv1alpha1.ContainerSource{},
		ReconcileKey: "non-existent-test-ns/non-existent-test-key",
		WantErr:      false,
	}, {
		Name:       "valid containersource, but sink does not exist",
		Reconciles: &sourcesv1alpha1.ContainerSource{},
		InitialState: []runtime.Object{
			getContainerSource(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, containerSourceName),
		WantErrMsg:   `sinks.duck.knative.dev "testsink" not found`,
	}, {
		Name:       "valid containersource, but sink is not sinkable",
		Reconciles: &sourcesv1alpha1.ContainerSource{},
		InitialState: []runtime.Object{
			getContainerSource_unsinkable(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, containerSourceName),
		Scheme:       scheme.Scheme,
		Objects: []runtime.Object{
			// An unsinkable resource
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": unsinkableAPIVersion,
					"kind":       unsinkableKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      unsinkableName,
					},
				},
			},
		},
		WantErrMsg: "sink does not contain sinkable",
	}, {
		Name:       "valid containersource, sink is sinkable",
		Reconciles: &sourcesv1alpha1.ContainerSource{},
		InitialState: []runtime.Object{
			getContainerSource(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, containerSourceName),
		Scheme:       scheme.Scheme,
		Objects: []runtime.Object{
			// k8s Service
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": sinkableAPIVersion,
					"kind":       sinkableKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      sinkableName,
					},
					"status": map[string]interface{}{
						"sinkable": map[string]interface{}{
							"domainInternal": sinkableDNS,
						},
					},
				},
			},
		},
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getContainerSource()
				s.Status.SinkURI = targetURI
				s.Status.InitializeConditions()
				s.Status.MarkDeploying("Deploying", "Created deployment %s", containerSourceName)
				s.Status.MarkSink(targetURI)
				return s
			}(),
		},
		IgnoreTimes: true,
	}, {
		Name:       "valid containersource, sink is provided",
		Reconciles: &sourcesv1alpha1.ContainerSource{},
		InitialState: []runtime.Object{
			func() runtime.Object {
				s := getContainerSource()
				s.Spec.Args = append(s.Spec.Args, fmt.Sprintf("--sink=%s", targetURI))
				return s
			}(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, containerSourceName),
		Scheme:       scheme.Scheme,
		Objects:      []runtime.Object{},
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getContainerSource()
				s.Spec.Args = append(s.Spec.Args, fmt.Sprintf("--sink=%s", targetURI))
				s.Status.SinkURI = targetURI
				s.Status.InitializeConditions()
				s.Status.MarkDeploying("Deploying", "Created deployment %s", containerSourceName)
				s.Status.MarkSink(targetURI)
				return s
			}(),
		},
		IgnoreTimes: true,
	}, {
		Name:       "valid containersource, sink is targetable",
		Reconciles: &sourcesv1alpha1.ContainerSource{},
		InitialState: []runtime.Object{
			getContainerSource(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, containerSourceName),
		Scheme:       scheme.Scheme,
		Objects: []runtime.Object{
			// k8s Service
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": sinkableAPIVersion,
					"kind":       sinkableKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      sinkableName,
					},
					"status": map[string]interface{}{
						"targetable": map[string]interface{}{
							"domainInternal": sinkableDNS,
						},
					},
				},
			},
		},
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getContainerSource()
				s.Status.SinkURI = targetURI
				s.Status.InitializeConditions()
				s.Status.MarkDeploying("Deploying", "Created deployment %s", containerSourceName)
				s.Status.MarkSink(targetURI)
				return s
			}(),
		},
		IgnoreTimes: true,
	}, /* TODO: support k8s service {
		Name:       "valid containersource, sink is a k8s service",
		Reconciles: &sourcesv1alpha1.ContainerSource{},
		InitialState: []runtime.Object{
			getContainerSource_sinkService(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, containerSourceName),
		Scheme:       scheme.Scheme,
		Objects: []runtime.Object{
			// sinkable
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": sinkServiceAPIVersion,
					"kind":       sinkServiceKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      sinkServiceName,
					},
				}},
		},
	},*/
}

func TestAllCases(t *testing.T) {
	recorder := record.NewBroadcaster().NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	for _, tc := range testCases {
		c := tc.GetClient()
		dc := tc.GetDynamicClient()

		r := &reconciler{
			client:        c,
			dynamicClient: dc,
			scheme:        tc.Scheme,
			restConfig:    &rest.Config{},
			recorder:      recorder,
		}
		t.Run(tc.Name, tc.RunnerSDK(t, r, c))
	}
}

func getContainerSource() *sourcesv1alpha1.ContainerSource {
	obj := &sourcesv1alpha1.ContainerSource{
		TypeMeta:   containerSourceType(),
		ObjectMeta: om(testNS, containerSourceName),
		Spec: sourcesv1alpha1.ContainerSourceSpec{
			Image: image,
			Args:  []string(nil),
			Sink: &corev1.ObjectReference{
				Name:       sinkableName,
				Kind:       sinkableKind,
				APIVersion: sinkableAPIVersion,
			},
		},
	}
	// selflink is not filled in when we create the object, so clear it
	obj.ObjectMeta.SelfLink = ""
	return obj
}

func getContainerSource_sinkService() *sourcesv1alpha1.ContainerSource {
	obj := &sourcesv1alpha1.ContainerSource{
		TypeMeta:   containerSourceType(),
		ObjectMeta: om(testNS, containerSourceName),
		Spec: sourcesv1alpha1.ContainerSourceSpec{
			Image: image,
			Args:  []string(nil),
			Sink: &corev1.ObjectReference{
				Name:       sinkServiceName,
				Kind:       sinkServiceKind,
				APIVersion: sinkServiceAPIVersion,
			},
		},
	}
	// selflink is not filled in when we create the object, so clear it
	obj.ObjectMeta.SelfLink = ""
	return obj
}

func getContainerSource_unsinkable() *sourcesv1alpha1.ContainerSource {
	obj := &sourcesv1alpha1.ContainerSource{
		TypeMeta:   containerSourceType(),
		ObjectMeta: om(testNS, containerSourceName),
		Spec: sourcesv1alpha1.ContainerSourceSpec{
			Image: image,
			Args:  []string{},
			Sink: &corev1.ObjectReference{
				Name:       unsinkableName,
				Kind:       unsinkableKind,
				APIVersion: unsinkableAPIVersion,
			},
		},
	}
	// selflink is not filled in when we create the object, so clear it
	obj.ObjectMeta.SelfLink = ""
	return obj
}
func containerSourceType() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: sourcesv1alpha1.SchemeGroupVersion.String(),
		Kind:       "ContainerSource",
	}
}

func om(namespace, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace: namespace,
		Name:      name,
		SelfLink:  fmt.Sprintf("/apis/eventing/sources/v1alpha1/namespaces/%s/object/%s", namespace, name),
	}
}

func getTestResources() []runtime.Object {
	return []runtime.Object{
		&unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": unsinkableAPIVersion,
				"kind":       unsinkableKind,
				"metadata": map[string]interface{}{
					"namespace": testNS,
					"name":      unsinkableName,
				},
			},
		}, &unstructured.Unstructured{
			Object: map[string]interface{}{
				"apiVersion": sinkableAPIVersion,
				"kind":       sinkableKind,
				"metadata": map[string]interface{}{
					"namespace": testNS,
					"name":      sinkableName,
				},
			},
		},
	}
}
