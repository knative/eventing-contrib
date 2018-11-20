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

package kuberneteseventsource

import (
	"context"
	"fmt"
	"github.com/google/go-cmp/cmp"
	sourcesv1alpha1 "github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	controllertesting "github.com/knative/eventing-sources/pkg/controller/testing"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"testing"
)

var (
	trueVal   = true
	targetURI = "http://sinkable.sink.svc.cluster.local/"
)

const (
	image               = "github.com/knative/test/image"
	sourceName          = "testkuberneteseventsource"
	testNS              = "testnamespace"
	testSAN             = "testserviceaccount"
	sourceUID           = "2a2208d1-ce67-11e8-b3a3-42010a8a00af"
	deployGeneratedName = "" //sad trombone

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
		Name:       "existing ready container source",
		Reconciles: &sourcesv1alpha1.KubernetesEventSource{},
		InitialState: []runtime.Object{
			func() runtime.Object {
				s := getSource()
				s.UID = sourceUID
				return s
			}(),
			func() runtime.Object {
				u := getContainerSource_isReady()
				u.SetOwnerReferences(getOwnerReferences())
				return u
			}(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, sourceName),
		Scheme:       scheme.Scheme,
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getSource()
				s.UID = sourceUID
				s.Status.InitializeConditions()

				c := getContainerSource_isReady()

				s.Status.PropagateContainerSourceStatus(c.Status)
				return s
			}(),
			func() runtime.Object {
				u := getContainerSource_isReady()
				u.SetOwnerReferences(getOwnerReferences())
				return u
			}(),
		},
		IgnoreTimes: true,
	}, {
		Name:       "existing container source, needs spec update",
		Reconciles: &sourcesv1alpha1.KubernetesEventSource{},
		InitialState: []runtime.Object{
			func() runtime.Object {
				s := getSource()
				s.UID = sourceUID
				return s
			}(),
			func() runtime.Object {
				u := getContainerSource_isReady()
				u.Spec.Image = "wrongImage"
				u.Spec.ServiceAccountName = "wrongServiceAccountName"
				u.Spec.Args = nil
				u.SetOwnerReferences(getOwnerReferences())
				return u
			}(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, sourceName),
		Scheme:       scheme.Scheme,
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getSource()
				s.UID = sourceUID
				s.Status.InitializeConditions()

				c := getContainerSource_isReady()

				s.Status.PropagateContainerSourceStatus(c.Status)
				return s
			}(),
			func() runtime.Object {
				u := getContainerSource_isReady()
				u.SetOwnerReferences(getOwnerReferences())
				return u
			}(),
		},
		IgnoreTimes: true,
	}, {
		Name:       "create new container source",
		Reconciles: &sourcesv1alpha1.KubernetesEventSource{},
		InitialState: []runtime.Object{
			func() runtime.Object {
				s := getSource()
				s.UID = sourceUID
				return s
			}(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, sourceName),
		Scheme:       scheme.Scheme,
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getSource()
				s.UID = sourceUID
				s.Status.InitializeConditions()

				c := getContainerSource()
				c.Status.InitializeConditions()

				s.Status.PropagateContainerSourceStatus(c.Status)
				return s
			}(),
			// TODO: can't test that the object exists because the mock does not
			// add type meta, and without type meta the mocks do not get the correct
			// object. Trash.
			//func() runtime.Object {
			//	u := getContainerSource()
			//	u.Status.InitializeConditions()
			//	u.SetOwnerReferences(getOwnerReferences())
			//	return u
			//}(),
		},
		IgnoreTimes: true,
	}, {
		Name:       "existing container source, not ready - deploying",
		Reconciles: &sourcesv1alpha1.KubernetesEventSource{},
		InitialState: []runtime.Object{
			func() runtime.Object {
				s := getSource()
				s.UID = sourceUID
				return s
			}(),
			func() runtime.Object {
				u := getContainerSource()
				u.Status.InitializeConditions()
				u.Status.MarkDeploying("reason", "message")
				u.SetOwnerReferences(getOwnerReferences())
				return u
			}(),
		},
		ReconcileKey: fmt.Sprintf("%s/%s", testNS, sourceName),
		Scheme:       scheme.Scheme,
		WantPresent: []runtime.Object{
			func() runtime.Object {
				s := getSource()
				s.UID = sourceUID
				s.Status.InitializeConditions()

				c := getContainerSource()
				c.Status.InitializeConditions()
				c.Status.MarkDeploying("reason", "message")

				s.Status.PropagateContainerSourceStatus(c.Status)
				return s
			}(),
			func() runtime.Object {
				u := getContainerSource()
				u.Status.InitializeConditions()
				u.Status.MarkDeploying("reason", "message")
				u.SetOwnerReferences(getOwnerReferences())
				return u
			}(),
		},
		IgnoreTimes: true,
	},
}

func TestAllCases(t *testing.T) {
	recorder := record.NewBroadcaster().NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	for _, tc := range testCases {
		c := tc.GetClient()
		dc := tc.GetDynamicClient()

		r := &reconciler{
			dynamicClient:       dc,
			scheme:              tc.Scheme,
			recorder:            recorder,
			receiveAdapterImage: image,
		}
		r.InjectClient(c)
		t.Run(tc.Name, tc.Runner(t, r, c))
	}
}

func getSource() *sourcesv1alpha1.KubernetesEventSource {
	obj := &sourcesv1alpha1.KubernetesEventSource{
		TypeMeta:   sourceType(),
		ObjectMeta: om(testNS, sourceName),
		Spec: sourcesv1alpha1.KubernetesEventSourceSpec{
			Namespace:          testNS,
			ServiceAccountName: testSAN,
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

func getContainerSource() *sourcesv1alpha1.ContainerSource {
	obj := &sourcesv1alpha1.ContainerSource{
		TypeMeta:   containerSourceType(),
		ObjectMeta: om(testNS, sourceName+"-abc"),
		Spec: sourcesv1alpha1.ContainerSourceSpec{
			Image: image,
			Args: []string{
				"--namespace=" + testNS,
			},
			ServiceAccountName: testSAN,
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

func getContainerSource_isReady() *sourcesv1alpha1.ContainerSource {
	obj := getContainerSource()
	obj.Status.InitializeConditions()
	obj.Status.MarkDeployed()
	obj.Status.MarkSink(targetURI)
	return obj
}

func sourceType() metav1.TypeMeta {
	return metav1.TypeMeta{
		APIVersion: sourcesv1alpha1.SchemeGroupVersion.String(),
		Kind:       "KubernetesEventSource",
	}
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

func getOwnerReferences() []metav1.OwnerReference {
	return []metav1.OwnerReference{{
		APIVersion:         sourcesv1alpha1.SchemeGroupVersion.String(),
		Kind:               "KubernetesEventSource",
		Name:               sourceName,
		Controller:         &trueVal,
		BlockOwnerDeletion: &trueVal,
		UID:                sourceUID,
	}}
}

// Direct Unit tests.

func TestObjectNotKubernetesEventSource(t *testing.T) {
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
	obj := getSource()

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
