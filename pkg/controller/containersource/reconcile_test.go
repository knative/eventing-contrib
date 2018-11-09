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
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/go-cmp/cmp"
	sourcesv1alpha1 "github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	"github.com/knative/eventing-sources/pkg/controller/containersource/resources"
	controllertesting "github.com/knative/eventing-sources/pkg/controller/testing"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"testing"
)

var (
	trueVal   = true
	targetURI = "http://sinkable.sink.svc.cluster.local/"
)

const (
	image               = "github.com/knative/test/image"
	containerSourceName = "testcontainersource"
	testNS              = "testnamespace"
	containerSourceUID  = "2a2208d1-ce67-11e8-b3a3-42010a8a00af"

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
		&duckv1alpha1.AddressableTypeList{},
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
		Name:       "valid containersource, but sink is not addressable",
		Reconciles: &sourcesv1alpha1.ContainerSource{},
		InitialState: []runtime.Object{
			getContainerSource_unsinkable(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, containerSourceName),
		Scheme:       scheme.Scheme,
		Objects: []runtime.Object{
			// unaddressable resource
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
		WantErrMsg: "sink does not contain address",
	}, {
		Name:       "valid containersource, sink is addressable",
		Reconciles: &sourcesv1alpha1.ContainerSource{},
		InitialState: []runtime.Object{
			getContainerSource(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, containerSourceName),
		Scheme:       scheme.Scheme,
		Objects: []runtime.Object{
			// Addressable resource
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": sinkableAPIVersion,
					"kind":       sinkableKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      sinkableName,
					},
					"status": map[string]interface{}{
						"address": map[string]interface{}{
							"hostname": sinkableDNS,
						},
					},
				},
			},
		},
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getContainerSource()
				s.Status.InitializeConditions()
				s.Status.MarkDeploying("Deploying", "Created deployment %s", containerSourceName)
				s.Status.MarkSink(targetURI)
				return s
			}(),
		},
		IgnoreTimes: true,
	}, {
		Name:       "valid containersource, sink is addressable, fields filled in",
		Reconciles: &sourcesv1alpha1.ContainerSource{},
		InitialState: []runtime.Object{
			getContainerSource_filledIn(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, containerSourceName),
		Scheme:       scheme.Scheme,
		Objects: []runtime.Object{
			// Addressable resource
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": sinkableAPIVersion,
					"kind":       sinkableKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      sinkableName,
					},
					"status": map[string]interface{}{
						"address": map[string]interface{}{
							"hostname": sinkableDNS,
						},
					},
				},
			},
		},
		WantPresent: []runtime.Object{
			getDeployment(getContainerSource_filledIn()),
		},
		IgnoreTimes: true,
	}, {
		Name:       "valid containersource, sink is Addressable but sink is nil",
		Reconciles: &sourcesv1alpha1.ContainerSource{},
		InitialState: []runtime.Object{
			getContainerSource(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, containerSourceName),
		Scheme:       scheme.Scheme,
		Objects: []runtime.Object{
			// Addressable resource
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": sinkableAPIVersion,
					"kind":       sinkableKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      sinkableName,
					},
					"status": map[string]interface{}{
						"address": map[string]interface{}(nil),
					},
				},
			},
		},
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getContainerSource()
				s.Status.InitializeConditions()
				s.Status.MarkNoSink("NotFound", "")
				return s
			}(),
		},
		IgnoreTimes: true,
		WantErrMsg:  `sink does not contain address`,
	}, {
		Name:       "invalid containersource, sink is nil",
		Reconciles: &sourcesv1alpha1.ContainerSource{},
		InitialState: []runtime.Object{
			func() runtime.Object {
				s := getContainerSource()
				s.Spec.Sink = nil
				return s
			}(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, containerSourceName),
		Scheme:       scheme.Scheme,
		Objects: []runtime.Object{
			// sinkable resource
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": sinkableAPIVersion,
					"kind":       sinkableKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      sinkableName,
					},
					"status": map[string]interface{}{
						"sinkable": map[string]interface{}(nil),
					},
				},
			},
		},
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getContainerSource()
				s.Spec.Sink = nil
				s.Status.InitializeConditions()
				s.Status.MarkNoSink("Missing", "")
				return s
			}(),
		},
		IgnoreTimes: true,
		WantErrMsg:  "Sink missing from spec",
	}, {
		Name:       "valid containersource, sink is provided",
		Reconciles: &sourcesv1alpha1.ContainerSource{},
		InitialState: []runtime.Object{
			func() runtime.Object {
				s := getContainerSource()
				s.Spec.Sink = nil
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
				s.Spec.Sink = nil
				s.Spec.Args = append(s.Spec.Args, fmt.Sprintf("--sink=%s", targetURI))
				s.Status.InitializeConditions()
				s.Status.MarkDeploying("Deploying", "Created deployment %s", containerSourceName)
				s.Status.MarkSink(targetURI)
				return s
			}(),
		},
		IgnoreTimes: true,
	}, {
		Name:       "valid containersource, sink, and deployment",
		Reconciles: &sourcesv1alpha1.ContainerSource{},
		InitialState: []runtime.Object{
			func() runtime.Object {
				s := getContainerSource()
				s.UID = containerSourceUID
				return s
			}(),
			func() runtime.Object {
				// TODO(n3wscott): this is very strange, I was not able to get
				// the fake client to return the resources.MakeDeployment version
				// back in the list call. I might have missed setting some special
				// metadata? Converting an unstructured and setting the fields
				// I care about did work.
				u := &unstructured.Unstructured{
					Object: map[string]interface{}{
						"apiVersion": "apps/v1",
						"kind":       "Deployment",
						"metadata": map[string]interface{}{
							"namespace": testNS,
							"name":      containerSourceName + "-abc",
						},
					},
				}
				u.SetOwnerReferences(getOwnerReferences())

				d := &appsv1.Deployment{}
				d.Status.ReadyReplicas = 1
				j, _ := u.MarshalJSON()
				json.Unmarshal(j, d)

				d1 := resources.MakeDeployment(nil, &resources.ContainerArguments{})
				d.Spec = d1.Spec
				return d
			}(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, containerSourceName),
		Scheme:       scheme.Scheme,
		Objects: []runtime.Object{
			// sinkable
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": sinkableAPIVersion,
					"kind":       sinkableKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      sinkableName,
						"uid":       containerSourceUID,
					},
					"status": map[string]interface{}{
						"address": map[string]interface{}{
							"hostname": sinkableDNS,
						},
					},
				},
			},
		},
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getContainerSource()
				s.UID = containerSourceUID
				s.Status.InitializeConditions()
				s.Status.MarkDeployed()
				s.Status.MarkSink(targetURI)
				return s
			}(),
		},
		IgnoreTimes: true,
	}, {
		Name:       "Error for create deployment",
		Reconciles: &sourcesv1alpha1.ContainerSource{},
		InitialState: []runtime.Object{
			func() runtime.Object {
				s := getContainerSource()
				s.UID = containerSourceUID
				return s
			}(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, containerSourceName),
		Scheme:       scheme.Scheme,
		Mocks: controllertesting.Mocks{
			MockCreates: []controllertesting.MockCreate{
				func(_ client.Client, _ context.Context, _ runtime.Object) (controllertesting.MockHandled, error) {
					return controllertesting.Handled, errors.New("force an error into client create")
				},
			},
		},
		Objects: []runtime.Object{
			// Addressable resource
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": sinkableAPIVersion,
					"kind":       sinkableKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      sinkableName,
					},
					"status": map[string]interface{}{
						"address": map[string]interface{}{
							"hostname": sinkableDNS,
						},
					},
				},
			},
		},
		IgnoreTimes: true,
		WantErrMsg:  "force an error into client create",
	}, {
		Name:       "Error for get source, other than not found",
		Reconciles: &sourcesv1alpha1.ContainerSource{},
		InitialState: []runtime.Object{
			func() runtime.Object {
				s := getContainerSource()
				s.UID = containerSourceUID
				return s
			}(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, containerSourceName),
		Scheme:       scheme.Scheme,
		Mocks: controllertesting.Mocks{
			MockLists: []controllertesting.MockList{
				func(_ client.Client, _ context.Context, _ *client.ListOptions, _ runtime.Object) (controllertesting.MockHandled, error) {
					return controllertesting.Handled, errors.New("force an error into client list")
				},
			},
		},
		Objects: []runtime.Object{
			// Addressable resource
			&unstructured.Unstructured{
				Object: map[string]interface{}{
					"apiVersion": sinkableAPIVersion,
					"kind":       sinkableKind,
					"metadata": map[string]interface{}{
						"namespace": testNS,
						"name":      sinkableName,
					},
					"status": map[string]interface{}{
						"address": map[string]interface{}{
							"hostname": sinkableDNS,
						},
					},
				},
			},
		},
		IgnoreTimes: true,
		WantErrMsg:  "force an error into client list",
	},
	/* TODO: support k8s service {
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
			dynamicClient: dc,
			scheme:        tc.Scheme,
			recorder:      recorder,
		}
		r.InjectClient(c)
		t.Run(tc.Name, tc.Runner(t, r, c))
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

func getContainerSource_filledIn() *sourcesv1alpha1.ContainerSource {
	obj := getContainerSource()
	obj.ObjectMeta.UID = containerSourceUID
	obj.Spec.Args = []string{"--foo", "bar"}
	obj.Spec.Env = []corev1.EnvVar{{Name: "FOO", Value: "bar"}}
	obj.Spec.ServiceAccountName = "foo"
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

func getDeployment(source *sourcesv1alpha1.ContainerSource) *appsv1.Deployment {
	sinkableURI := fmt.Sprintf("http://%s/", sinkableDNS)
	args := append(source.Spec.Args, fmt.Sprintf("--sink=%s", sinkableURI))
	env := append(source.Spec.Env, corev1.EnvVar{Name: "SINK", Value: sinkableURI})
	return &appsv1.Deployment{
		TypeMeta: deploymentType(),
		ObjectMeta: metav1.ObjectMeta{
			GenerateName:    fmt.Sprintf("%s-", source.Name),
			Namespace:       source.Namespace,
			OwnerReferences: getOwnerReferences(),
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"source": source.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{
						"sidecar.istio.io/inject": "true",
					},
					Labels: map[string]string{
						"source": source.Name,
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:            "source",
						Image:           source.Spec.Image,
						Args:            args,
						Env:             env,
						ImagePullPolicy: corev1.PullIfNotPresent,
					}},
					ServiceAccountName: source.Spec.ServiceAccountName,
				},
			},
		},
	}
}

func containerSourceType() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: sourcesv1alpha1.SchemeGroupVersion.String(),
		Kind:       "ContainerSource",
	}
}

func deploymentType() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: appsv1.SchemeGroupVersion.String(),
		Kind:       "Deployment",
	}
}

func om(namespace, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace: namespace,
		Name:      name,
		SelfLink:  fmt.Sprintf("/apis/eventing/sources/v1alpha1/namespaces/%s/object/%s", namespace, name),
	}
}

func getOwnerReferences() []metav1.OwnerReference {
	return []metav1.OwnerReference{{
		APIVersion:         sourcesv1alpha1.SchemeGroupVersion.String(),
		Kind:               "ContainerSource",
		Name:               containerSourceName,
		Controller:         &trueVal,
		BlockOwnerDeletion: &trueVal,
		UID:                containerSourceUID,
	}}
}

// Direct Unit tests.

func TestObjectNotContainerSource(t *testing.T) {
	r := reconciler{}
	obj := &corev1.ObjectReference{
		Name:       unsinkableName,
		Kind:       unsinkableKind,
		APIVersion: unsinkableAPIVersion,
	}

	got, gotErr := r.Reconcile(context.TODO(), obj)
	var want runtime.Object = obj
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected returned object (-want, +got) = %v", diff)
	}
	var wantErr error = nil
	if diff := cmp.Diff(wantErr, gotErr); diff != "" {
		t.Errorf("unexpected returned error (-want, +got) = %v", diff)
	}
}

func TestObjectHasDeleteTimestamp(t *testing.T) {
	r := reconciler{}
	obj := getContainerSource()

	now := metav1.Now()
	obj.DeletionTimestamp = &now
	got, gotErr := r.Reconcile(context.TODO(), obj)
	var want runtime.Object = obj
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("unexpected returned object (-want, +got) = %v", diff)
	}
	var wantErr error = nil
	if diff := cmp.Diff(wantErr, gotErr); diff != "" {
		t.Errorf("unexpected returned error (-want, +got) = %v", diff)
	}
}

func TestInjectConfig(t *testing.T) {
	r := reconciler{}

	r.InjectConfig(&rest.Config{})

	if r.dynamicClient == nil {
		t.Errorf("dynamicClient was nil but expected non nil")
	}
}
