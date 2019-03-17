/*
Copyright 2019 The Knative Authors

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

package reconciler

import (
	"fmt"
	"testing"

	camelv1alpha1 "github.com/apache/camel-k/pkg/apis/camel/v1alpha1"
	sourcesv1alpha1 "github.com/knative/eventing-sources/contrib/camel/pkg/apis/sources/v1alpha1"
	"github.com/knative/eventing-sources/contrib/camel/pkg/reconciler/resources"
	genericv1alpha1 "github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	controllertesting "github.com/knative/eventing-sources/pkg/controller/testing"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"go.uber.org/zap"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
)

var (
	// deletionTime is used when objects are marked as deleted. Rfc3339Copy()
	// truncates to seconds to match the loss of precision during serialization.
	deletionTime = metav1.Now().Rfc3339Copy()

	trueVal = true
)

const (
	alternativeImage       = "apache/camel-k-base-alternative"
	alternativeContextName = "alternative-context"

	sourceName = "test-camel-source"
	sourceUID  = "1234-5678-90"
	testNS     = "testnamespace"

	addressableName       = "testsink"
	addressableKind       = "Sink"
	addressableAPIVersion = "duck.knative.dev/v1alpha1"
	addressableDNS        = "addressable.sink.svc.cluster.local"
	addressableURI        = "http://addressable.sink.svc.cluster.local/"
)

func init() {
	// Add types to scheme
	addToScheme(
		v1.AddToScheme,
		corev1.AddToScheme,
		sourcesv1alpha1.SchemeBuilder.AddToScheme,
		genericv1alpha1.SchemeBuilder.AddToScheme,
		duckv1alpha1.AddToScheme,
		camelv1alpha1.SchemeBuilder.AddToScheme,
	)
}

func addToScheme(funcs ...func(*runtime.Scheme) error) {
	for _, fun := range funcs {
		if err := fun(scheme.Scheme); err != nil {
			panic(fmt.Errorf("error during scheme registration: %v", zap.Error(err)))
		}
	}
}

func TestReconcile(t *testing.T) {
	testCases := []controllertesting.TestCase{
		{
			Name: "Deleted source",
			InitialState: []runtime.Object{
				getDeletedSource(),
			},
			WantPresent: []runtime.Object{
				getDeletedSource(),
			},
			WantAbsent: []runtime.Object{
				getPlatform(),
				getContext(),
			},
		},
		{
			Name: "Cannot get sink URI",
			InitialState: []runtime.Object{
				getPlatform(),
				getSource(),
			},
			WantPresent: []runtime.Object{
				getSourceWithNoSink(),
			},
			WantAbsent: []runtime.Object{
				getContext(),
			},
			WantErrMsg: "sinks.duck.knative.dev \"testsink\" not found",
		},
		{
			Name: "Creating integration",
			InitialState: []runtime.Object{
				getPlatform(),
				getSource(),
				getAddressable(),
			},
			WantPresent: []runtime.Object{
				getDeployingSource(),
			},
			WantAbsent: []runtime.Object{
				getContext(),
			},
		},
		{
			Name: "Source Deployed",
			InitialState: []runtime.Object{
				getPlatform(),
				getSource(),
				getAddressable(),
				getRunningIntegration(t),
			},
			WantPresent: []runtime.Object{
				getDeployedSource(),
				getRunningIntegration(t),
			},
			WantAbsent: []runtime.Object{
				getContext(),
			},
		},
		{
			Name: "Source changed",
			InitialState: []runtime.Object{
				getPlatform(),
				getSource(),
				getAddressable(),
				getWrongIntegration(t),
			},
			WantPresent: []runtime.Object{
				getSourceWithUpdatingIntegration(),
				getRunningIntegration(t),
			},
			WantAbsent: []runtime.Object{
				getContext(),
			},
		},
		{
			Name: "Source with image",
			InitialState: []runtime.Object{
				getPlatform(),
				withAlternativeImage(getSource()),
				getAddressable(),
				getRunningIntegration(t),
			},
			WantPresent: []runtime.Object{
				withAlternativeImage(getDeployedSource()),
				getRunningIntegration(t),
				getContext(),
			},
		},
		{
			Name: "Source with image and existing context",
			InitialState: []runtime.Object{
				getPlatform(),
				withAlternativeImage(getSource()),
				getAddressable(),
				getRunningIntegrationWithAlternativeContext(t),
				getAlternativeContext(),
			},
			WantPresent: []runtime.Object{
				withAlternativeImage(getDeployedSource()),
				getRunningIntegrationWithAlternativeContext(t),
				getAlternativeContext(),
			},
		},
		{
			Name: "Source with image and unbound existing context",
			InitialState: []runtime.Object{
				getPlatform(),
				withAlternativeImage(getSource()),
				getAddressable(),
				getRunningIntegration(t),
				getAlternativeContext(),
			},
			WantPresent: []runtime.Object{
				withAlternativeImage(getSourceWithUpdatingIntegration()),
				getRunningIntegrationWithAlternativeContext(t),
				getAlternativeContext(),
			},
		},
		{
			Name: "Source without platform",
			InitialState: []runtime.Object{
				getSource(),
				getAddressable(),
				getRunningIntegration(t),
			},
			WantPresent: []runtime.Object{
				getDeployedSource(),
				getRunningIntegration(t),
				getPlatform(),
			},
			WantAbsent: []runtime.Object{
				getContext(),
			},
		},
	}
	for _, tc := range testCases {
		recorder := record.NewBroadcaster().NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})
		tc.IgnoreTimes = true
		tc.ReconcileKey = fmt.Sprintf("%s/%s", testNS, sourceName)
		tc.Reconciles = getSource()
		tc.Scheme = scheme.Scheme

		c := tc.GetClient()
		r := &reconciler{
			client:   c,
			scheme:   tc.Scheme,
			recorder: recorder,
		}
		if err := r.InjectClient(c); err != nil {
			t.Errorf("cannot inject client: %v", zap.Error(err))
		}

		t.Run(tc.Name, tc.Runner(t, r, c))
	}
}

func getPlatform() runtime.Object {
	return resources.MakePlatform(testNS)
}

func getSource() *sourcesv1alpha1.CamelSource {
	obj := &sourcesv1alpha1.CamelSource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: sourcesv1alpha1.SchemeGroupVersion.String(),
			Kind:       "CamelSource",
		},
		ObjectMeta: om(testNS, sourceName),
		Spec: sourcesv1alpha1.CamelSourceSpec{
			Source: sourcesv1alpha1.CamelSourceOriginSpec{
				Component: &sourcesv1alpha1.CamelSourceOriginComponentSpec{
					URI: "timer:tick?period=3s",
				},
			},
			Sink: &corev1.ObjectReference{
				Name:       addressableName,
				Kind:       addressableKind,
				APIVersion: addressableAPIVersion,
			},
		},
	}
	// selflink is not filled in when we create the object, so clear it
	obj.ObjectMeta.SelfLink = ""
	return obj
}

func withAlternativeImage(src *sourcesv1alpha1.CamelSource) *sourcesv1alpha1.CamelSource {
	src.Spec.Image = alternativeImage
	return src
}

func getContext() *camelv1alpha1.IntegrationContext {
	return resources.MakeContext(testNS, alternativeImage)
}

func getAlternativeContext() *camelv1alpha1.IntegrationContext {
	ctx := resources.MakeContext(testNS, alternativeImage)
	ctx.Name = alternativeContextName
	return ctx
}

func getRunningIntegration(t *testing.T) *camelv1alpha1.Integration {
	flows := camelv1alpha1.Flows{
		{
			Steps: []camelv1alpha1.Step{
				{
					Kind: "endpoint",
					URI:  "timer:tick?period=3s",
				},
				{
					Kind: "endpoint",
					URI:  "knative:endpoint/sink",
				},
			},
		},
	}
	source, err := flows.Serialize()
	if err != nil {
		t.Error("failed to serialize source", err)
		return nil
	}

	it, err := resources.MakeIntegration(&resources.CamelArguments{
		Name:      sourceName,
		Namespace: testNS,
		Sink:      addressableURI,
		Source: resources.CamelArgumentsSource{
			Name:    "source.flow",
			Content: source,
		},
	})
	if err != nil {
		t.Error("failed to create integration", err)
	}
	it.ObjectMeta.OwnerReferences = getOwnerReferences()
	it.Status.Phase = camelv1alpha1.IntegrationPhaseRunning
	return it
}

func getRunningIntegrationWithAlternativeContext(t *testing.T) *camelv1alpha1.Integration {
	it := getRunningIntegration(t)
	it.Spec.Context = alternativeContextName
	return it
}

func getWrongIntegration(t *testing.T) *camelv1alpha1.Integration {
	it := getRunningIntegration(t)
	it.Spec.Sources[0].Content = "wrong"
	return it
}

func getSourceWithNoSink() *sourcesv1alpha1.CamelSource {
	src := getSource()
	src.Status.InitializeConditions()
	src.Status.MarkNoSink("NotFound", "")
	return src
}

func getSourceWithUpdatingIntegration() *sourcesv1alpha1.CamelSource {
	src := getSource()
	src.Status.InitializeConditions()
	src.Status.MarkSink(addressableURI)
	src.Status.MarkDeploying("IntegrationUpdated", "Updated integration ")
	return src
}

func getDeployingSource() *sourcesv1alpha1.CamelSource {
	src := getSource()
	src.Status.InitializeConditions()
	src.Status.MarkSink(addressableURI)
	src.Status.MarkDeploying("Deploying", "Created integration ")
	return src
}

func getDeployedSource() *sourcesv1alpha1.CamelSource {
	src := getSource()
	src.Status.InitializeConditions()
	src.Status.MarkSink(addressableURI)
	src.Status.MarkDeployed()
	return src
}

func getDeletedSource() *sourcesv1alpha1.CamelSource {
	src := getSource()
	src.ObjectMeta.DeletionTimestamp = &deletionTime
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

func getAddressable() *unstructured.Unstructured {
	return &unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": addressableAPIVersion,
			"kind":       addressableKind,
			"metadata": map[string]interface{}{
				"namespace": testNS,
				"name":      addressableName,
			},
			"status": map[string]interface{}{
				"address": map[string]interface{}{
					"hostname": addressableDNS,
				},
			},
		},
	}
}

func getOwnerReferences() []metav1.OwnerReference {
	return []metav1.OwnerReference{{
		APIVersion:         sourcesv1alpha1.SchemeGroupVersion.String(),
		Kind:               "CamelSource",
		Name:               sourceName,
		Controller:         &trueVal,
		BlockOwnerDeletion: &trueVal,
		UID:                sourceUID,
	}}
}
