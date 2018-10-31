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

package gcppubsub

import (
	"context"
	"fmt"

	"sigs.k8s.io/controller-runtime/pkg/client"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"cloud.google.com/go/pubsub"

	"github.com/pkg/errors"

	"k8s.io/client-go/rest"

	//"fmt"
	"testing"

	sourcesv1alpha1 "github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	controllertesting "github.com/knative/eventing-sources/pkg/controller/testing"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	//"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	//"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var (
	// deletionTime is used when objects are marked as deleted. Rfc3339Copy()
	// truncates to seconds to match the loss of precision during serialization.
	deletionTime = metav1.Now().Rfc3339Copy()

	trueVal = true
)

const (
	raImage    = "test-ra-image"
	raSvcAccnt = "test-ra-service-account"

	pscData = "pubSubClientCreatorData"

	image      = "github.com/knative/test/image"
	sourceName = "test-gcp-pub-sub-source"
	sourceUID  = "1234-5678-90"
	testNS     = "testnamespace"

	sinkableName       = "testsink"
	sinkableKind       = "Sink"
	sinkableAPIVersion = "duck.knative.dev/v1alpha1"
	sinkableDNS        = "sinkable.sink.svc.cluster.local"
	sinkableURI        = "http://sinkable.sink.svc.cluster.local/"
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
	v1.AddToScheme(scheme.Scheme)
	corev1.AddToScheme(scheme.Scheme)
	sourcesv1alpha1.SchemeBuilder.AddToScheme(scheme.Scheme)
	duckv1alpha1.AddToScheme(scheme.Scheme)
	duckAddKnownTypes(scheme.Scheme)
}

type pubSubClientCreatorData struct {
	clientCreateErr error
	subExists       bool
	subExistsErr    error
	createSubErr    error
	deleteSubErr    error
}

func TestInjectConfig(t *testing.T) {
	r := reconciler{}

	r.InjectConfig(&rest.Config{})

	if r.dynamicClient == nil {
		t.Errorf("dynamicClient was nil but expected non nil")
	}
}

func TestReconcile(t *testing.T) {
	testCases := []controllertesting.TestCase{
		{
			Name: "not a GCP PubSub source",
			// This is not a GcpPubSubSource.
			Reconciles: getNonGcpPubSubSource(),
			InitialState: []runtime.Object{
				getNonGcpPubSubSource(),
			},
		}, {
			Name: "deleting - cannot create client",
			InitialState: []runtime.Object{
				getDeletingSource(),
			},
			OtherTestData: map[string]interface{}{
				pscData: pubSubClientCreatorData{
					clientCreateErr: errors.New("test-induced-error"),
				},
			},
			WantErrMsg: "test-induced-error",
		}, {
			Name: "deleting - error checking subscription exists",
			InitialState: []runtime.Object{
				getDeletingSource(),
			},
			OtherTestData: map[string]interface{}{
				pscData: pubSubClientCreatorData{
					subExistsErr: errors.New("test-induced-error"),
				},
			},
			WantErrMsg: "test-induced-error",
		}, {
			Name: "deleting - cannot delete subscription",
			InitialState: []runtime.Object{
				getDeletingSource(),
			},
			OtherTestData: map[string]interface{}{
				pscData: pubSubClientCreatorData{
					subExists:    true,
					deleteSubErr: errors.New("test-induced-error"),
				},
			},
			WantErrMsg: "test-induced-error",
		}, {
			Name: "deleting - remove finalizer",
			InitialState: []runtime.Object{
				getDeletingSource(),
			},
			WantPresent: []runtime.Object{
				getDeletingSourceWithoutFinalizer(),
			},
		}, {
			Name: "cannot get sinkURI",
			InitialState: []runtime.Object{
				getSource(),
			},
			WantPresent: []runtime.Object{
				getSourceWithFinalizerAndNoSink(),
			},
			WantErrMsg: "sinks.duck.knative.dev \"testsink\" not found",
		}, {
			Name: "cannot create client",
			InitialState: []runtime.Object{
				getSource(),
			},
			Objects: getSinkable(),
			OtherTestData: map[string]interface{}{
				pscData: pubSubClientCreatorData{
					clientCreateErr: errors.New("test-induced-error"),
				},
			},
			WantPresent: []runtime.Object{
				getSourceWithFinalizerAndSink(),
			},
			WantErrMsg: "test-induced-error",
		}, {
			Name: "error checking subscription exists",
			InitialState: []runtime.Object{
				getSource(),
			},
			Objects: getSinkable(),
			OtherTestData: map[string]interface{}{
				pscData: pubSubClientCreatorData{
					subExistsErr: errors.New("test-induced-error"),
				},
			},
			WantPresent: []runtime.Object{
				getSourceWithFinalizerAndSink(),
			},
			WantErrMsg: "test-induced-error",
		}, {
			Name: "cannot create subscription",
			InitialState: []runtime.Object{
				getSource(),
			},
			Objects: getSinkable(),
			OtherTestData: map[string]interface{}{
				pscData: pubSubClientCreatorData{
					createSubErr: errors.New("test-induced-error"),
				},
			},
			WantPresent: []runtime.Object{
				getSourceWithFinalizerAndSink(),
			},
			WantErrMsg: "test-induced-error",
		}, {
			Name: "reusing existing subscription - cannot create receive adapter",
			InitialState: []runtime.Object{
				getSource(),
			},
			Objects: getSinkable(),
			Mocks: controllertesting.Mocks{
				MockCreates: []controllertesting.MockCreate{
					func(_ client.Client, _ context.Context, _ runtime.Object) (controllertesting.MockHandled, error) {
						return controllertesting.Handled, errors.New("test-induced-error")
					},
				},
			},
			OtherTestData: map[string]interface{}{
				pscData: pubSubClientCreatorData{
					subExists:    true,
					createSubErr: errors.New("some other error that is not seen, because it is not created"),
				},
			},
			WantPresent: []runtime.Object{
				getSourceWithFinalizerAndSinkAndSubscribed(),
			},
			WantErrMsg: "test-induced-error",
		}, {
			Name: "cannot create receive adapter",
			InitialState: []runtime.Object{
				getSource(),
			},
			Objects: getSinkable(),
			Mocks: controllertesting.Mocks{
				MockCreates: []controllertesting.MockCreate{
					func(_ client.Client, _ context.Context, _ runtime.Object) (controllertesting.MockHandled, error) {
						return controllertesting.Handled, errors.New("test-induced-error")
					},
				},
			},
			WantPresent: []runtime.Object{
				getSourceWithFinalizerAndSinkAndSubscribed(),
			},
			WantErrMsg: "test-induced-error",
		}, {
			Name: "cannot list deployments",
			InitialState: []runtime.Object{
				getSource(),
			},
			Objects: getSinkable(),
			Mocks: controllertesting.Mocks{
				MockLists: []controllertesting.MockList{
					func(_ client.Client, _ context.Context, _ *client.ListOptions, _ runtime.Object) (controllertesting.MockHandled, error) {
						return controllertesting.Handled, errors.New("test-induced-error")
					},
				},
			},
			WantPresent: []runtime.Object{
				getSourceWithFinalizerAndSinkAndSubscribed(),
			},
			WantErrMsg: "test-induced-error",
		}, {
			Name: "successful create",
			InitialState: []runtime.Object{
				getSource(),
			},
			Objects: getSinkable(),
			WantPresent: []runtime.Object{
				getReadySource(),
			},
		}, {
			Name: "successful create - reuse existing receive adapter",
			InitialState: []runtime.Object{
				getSource(),
				getReceiveAdapter(),
			},
			Objects: getSinkable(),
			Mocks: controllertesting.Mocks{
				MockCreates: []controllertesting.MockCreate{
					func(_ client.Client, _ context.Context, _ runtime.Object) (controllertesting.MockHandled, error) {
						return controllertesting.Handled, errors.New("an error that won't be seen because create is not called")
					},
				},
			},
			WantPresent: []runtime.Object{
				getReadySource(),
			},
		},
	}
	for _, tc := range testCases {
		tc.IgnoreTimes = true
		tc.ReconcileKey = fmt.Sprintf("%s/%s", testNS, sourceName)
		if tc.Reconciles == nil {
			tc.Reconciles = getSource()
		}
		tc.Scheme = scheme.Scheme

		c := tc.GetClient()
		r := &reconciler{
			client:        c,
			dynamicClient: tc.GetDynamicClient(),
			scheme:        tc.Scheme,

			pubSubClientCreator: createPubSubClientCreator(tc.OtherTestData[pscData]),

			receiveAdapterImage:              raImage,
			receiveAdapterServiceAccountName: raSvcAccnt,
		}
		r.InjectClient(c)
		t.Run(tc.Name, tc.Runner(t, r, c))
	}
}

func getNonGcpPubSubSource() *sourcesv1alpha1.ContainerSource {
	obj := &sourcesv1alpha1.ContainerSource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: sourcesv1alpha1.SchemeGroupVersion.String(),
			Kind:       "ContainerSource",
		},
		ObjectMeta: om(testNS, sourceName),
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

func getSource() *sourcesv1alpha1.GcpPubSubSource {
	obj := &sourcesv1alpha1.GcpPubSubSource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: sourcesv1alpha1.SchemeGroupVersion.String(),
			Kind:       "GcpPubSubSource",
		},
		ObjectMeta: om(testNS, sourceName),
		Spec: sourcesv1alpha1.GcpPubSubSourceSpec{
			GcpCredsSecret: corev1.SecretKeySelector{
				LocalObjectReference: corev1.LocalObjectReference{
					Name: "cred-secret",
				},
				Key: "secret-entry.json",
			},
			GoogleCloudProject: "my-gcp-project",
			Topic:              "laconia",
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

func getDeletingSourceWithoutFinalizer() *sourcesv1alpha1.GcpPubSubSource {
	src := getSource()
	src.DeletionTimestamp = &deletionTime
	return src
}

func getDeletingSource() *sourcesv1alpha1.GcpPubSubSource {
	src := getDeletingSourceWithoutFinalizer()
	src.Finalizers = []string{finalizerName}
	return src
}

func getSourceWithFinalizer() *sourcesv1alpha1.GcpPubSubSource {
	src := getSource()
	src.Finalizers = []string{finalizerName}
	src.Status.InitializeConditions()
	return src
}

func getSourceWithFinalizerAndNoSink() *sourcesv1alpha1.GcpPubSubSource {
	src := getSourceWithFinalizer()
	src.Status.MarkNoSink("NotFound", "")
	return src
}

func getSourceWithFinalizerAndSink() *sourcesv1alpha1.GcpPubSubSource {
	src := getSourceWithFinalizer()
	src.Status.MarkSink(sinkableURI)
	return src
}

func getSourceWithFinalizerAndSinkAndSubscribed() *sourcesv1alpha1.GcpPubSubSource {
	src := getSourceWithFinalizerAndSink()
	src.Status.MarkSubscribed()
	return src
}

func getReadySource() *sourcesv1alpha1.GcpPubSubSource {
	src := getSourceWithFinalizerAndSinkAndSubscribed()
	src.Status.MarkDeployed()
	return src
}

func om(namespace, name string) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace: namespace,
		Name:      name,
		SelfLink:  fmt.Sprintf("/apis/eventing/sources/v1alpha1/namespaces/%s/object/%s", namespace, name),
		UID:       sourceUID,
	}
}

func createPubSubClientCreator(value interface{}) pubSubClientCreator {
	var data pubSubClientCreatorData
	var ok bool
	if data, ok = value.(pubSubClientCreatorData); !ok {
		data = pubSubClientCreatorData{}
	}
	if data.clientCreateErr != nil {
		return func(_ context.Context, _ string) (pubSubClient, error) {
			return nil, data.clientCreateErr
		}
	}
	return func(_ context.Context, _ string) (pubSubClient, error) {
		return &fakePubSubClient{
			data: data,
		}, nil
	}
}

func getSinkable() []runtime.Object {
	return []runtime.Object{
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
					"sinkable": map[string]interface{}{
						"domainInternal": sinkableDNS,
					},
				},
			},
		},
	}
}

func getReceiveAdapter() *v1.Deployment {
	return &v1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			OwnerReferences: []metav1.OwnerReference{
				{
					Controller: &trueVal,
					UID:        sourceUID,
				},
			},
			Labels:    getLabels(getSource()),
			Namespace: testNS,
		},
		TypeMeta: metav1.TypeMeta{
			APIVersion: v1.SchemeGroupVersion.String(),
			Kind:       "Deployment",
		},
	}
}

type fakePubSubClient struct {
	data pubSubClientCreatorData
}

func (c *fakePubSubClient) SubscriptionInProject(id, projectId string) pubSubSubscription {
	return &fakePubSubSubscription{
		data: c.data,
	}
}

func (c *fakePubSubClient) CreateSubscription(ctx context.Context, id string, cfg pubsub.SubscriptionConfig) (pubSubSubscription, error) {
	if c.data.createSubErr != nil {
		return nil, c.data.createSubErr
	}
	return &fakePubSubSubscription{
		data: c.data,
	}, nil
}

func (c *fakePubSubClient) Topic(name string) *pubsub.Topic {
	return &pubsub.Topic{}
}

type fakePubSubSubscription struct {
	data pubSubClientCreatorData
}

func (s *fakePubSubSubscription) Exists(ctx context.Context) (bool, error) {
	return s.data.subExists, s.data.subExistsErr
}

func (s *fakePubSubSubscription) ID() string {
	return "fake-pub-sub-id"
}

func (s *fakePubSubSubscription) Delete(ctx context.Context) error {
	return s.data.deleteSubErr
}
