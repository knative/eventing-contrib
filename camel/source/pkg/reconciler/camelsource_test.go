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
	"context"
	"fmt"
	"strconv"
	"testing"

	camelv1 "github.com/apache/camel-k/pkg/apis/camel/v1"
	"go.uber.org/zap"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	sourcesv1alpha1 "knative.dev/eventing-contrib/camel/source/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing-contrib/camel/source/pkg/reconciler/resources"
	controllertesting "knative.dev/eventing-contrib/pkg/controller/testing"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/pkg/resolver"
)

var (
	// deletionTime is used when objects are marked as deleted. Rfc3339Copy()
	// truncates to seconds to match the loss of precision during serialization.
	deletionTime = metav1.Now().Rfc3339Copy()

	trueVal = true
)

const (
	alternativeImage   = "apache/camel-k-base-alternative"
	alternativeKitName = "alternative-kit"

	sourceName = "test-camel-source"
	sourceUID  = "1234-5678-90"
	testNS     = "testnamespace"
	generation = 1

	addressableName       = "testsink"
	addressableKind       = "Service"
	addressableAPIVersion = "serving.knative.dev/v1"
	addressableDNS        = "addressable.sink.svc.cluster.local"
	addressableURI        = "http://addressable.sink.svc.cluster.local"
)

func init() {
	// Add types to scheme
	addToScheme(
		v1.AddToScheme,
		corev1.AddToScheme,
		sourcesv1alpha1.SchemeBuilder.AddToScheme,
		duckv1alpha1.AddToScheme,
		duckv1beta1.AddToScheme,
		camelv1.SchemeBuilder.AddToScheme,
		duckv1.SchemeBuilder.AddToScheme,
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
				// ResourceVersion needs to be bumped because of the change
				// https://github.com/kubernetes-sigs/controller-runtime/pull/620
				withBumpedResourceVersion(getDeletedSource()),
			},
			WantAbsent: []runtime.Object{
				getContext(),
			},
		},
		{
			Name: "Cannot get sink URI",
			InitialState: []runtime.Object{
				getSource(),
			},
			WantPresent: []runtime.Object{
				func() runtime.Object {
					s := withBumpedResourceVersion(getSourceWithNoSink())
					s.Status.ObservedGeneration = generation
					return s
				}(),
			},
			WantAbsent: []runtime.Object{
				getContext(),
			},
			WantErrMsg: `failed to get ref &ObjectReference{Kind:Service,Namespace:testnamespace,Name:testsink,UID:,APIVersion:serving.knative.dev/v1,ResourceVersion:,FieldPath:,}: services.serving.knative.dev "testsink" not found`,
		},
		{
			Name: "Creating integration",
			InitialState: []runtime.Object{
				getSource(),
				getAddressable(),
			},
			WantPresent: []runtime.Object{
				func() runtime.Object {
					s := withBumpedResourceVersion(getDeployingSource())
					s.Status.ObservedGeneration = generation
					return s
				}(),
			},
			WantAbsent: []runtime.Object{
				getContext(),
			},
		},
		{
			Name: "Source Deployed",
			InitialState: []runtime.Object{
				getSource(),
				getAddressable(),
				getRunningIntegration(t),
			},
			WantPresent: []runtime.Object{
				func() runtime.Object {
					s := withBumpedResourceVersion(asDeployedSource(getSource()))
					s.Status.ObservedGeneration = generation
					return s
				}(),
				getRunningIntegration(t),
			},
			WantAbsent: []runtime.Object{
				getContext(),
			},
		},
		{
			Name:       "Camel K Source Deployed",
			Reconciles: getCamelKSource(),
			InitialState: []runtime.Object{
				getCamelKSource(),
				getAddressable(),
				getRunningCamelKIntegration(t),
			},
			WantPresent: []runtime.Object{
				func() runtime.Object {
					s := withBumpedResourceVersion(asDeployedSource(getCamelKSource()))
					s.Status.ObservedGeneration = generation
					return s
				}(),
				getRunningCamelKIntegration(t),
			},
			WantAbsent: []runtime.Object{
				getContext(),
			},
		},
		{
			Name: "Source changed",
			InitialState: []runtime.Object{
				getSource(),
				getAddressable(),
				getWrongIntegration(t),
			},
			WantPresent: []runtime.Object{
				func() runtime.Object {
					s := withBumpedResourceVersion(withUpdatingIntegration(getSource()))
					s.Status.ObservedGeneration = generation
					return s
				}(),
				withBumpedResourceVersionIn(getRunningIntegration(t)),
			},
			WantAbsent: []runtime.Object{
				getContext(),
			},
		},
		{
			Name:       "Camel K Source changed",
			Reconciles: getCamelKSource(),
			InitialState: []runtime.Object{
				getCamelKSource(),
				getAddressable(),
				getWrongIntegration(t),
			},
			WantPresent: []runtime.Object{
				func() runtime.Object {
					s := withBumpedResourceVersion(withUpdatingIntegration(getCamelKSource()))
					s.Status.ObservedGeneration = generation
					return s
				}(),
				withBumpedResourceVersionIn(getRunningCamelKIntegration(t)),
			},
			WantAbsent: []runtime.Object{
				getContext(),
			},
		},
		{
			Name: "Source with context",
			InitialState: []runtime.Object{
				withAlternativeContext(getSource()),
				getAddressable(),
				integrationWithAlternativeContext(getRunningIntegration(t)),
				getAlternativeContext(),
			},
			WantPresent: []runtime.Object{
				func() runtime.Object {
					s := withBumpedResourceVersion(withAlternativeContext(asDeployedSource(getSource())))
					s.Status.ObservedGeneration = generation
					return s
				}(),
				integrationWithAlternativeContext(getRunningIntegration(t)),
				getAlternativeContext(),
			},
		},
		{
			Name:       "Camel K Source with context",
			Reconciles: withAlternativeContext(getCamelKSource()),
			InitialState: []runtime.Object{
				withAlternativeContext(getCamelKSource()),
				getAddressable(),
				integrationWithAlternativeContext(getRunningCamelKIntegration(t)),
				getAlternativeContext(),
			},
			WantPresent: []runtime.Object{
				func() runtime.Object {
					s := withBumpedResourceVersion(withAlternativeContext(asDeployedSource(getCamelKSource())))
					s.Status.ObservedGeneration = generation
					return s
				}(),
				integrationWithAlternativeContext(getRunningCamelKIntegration(t)),
				getAlternativeContext(),
			},
		},
		{
			Name:       "Camel K Flow source",
			Reconciles: getCamelKFlowSource(),
			InitialState: []runtime.Object{
				getCamelKFlowSource(),
				getAddressable(),
				getRunningCamelKFlowIntegration(t),
			},
			WantPresent: []runtime.Object{
				func() runtime.Object {
					s := withBumpedResourceVersion(asDeployedSource(getCamelKFlowSource()))
					s.Status.ObservedGeneration = generation
					return s
				}(),
				getRunningCamelKFlowIntegration(t),
			},
		},
	}
	for _, tc := range testCases {
		recorder := record.NewBroadcaster().NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})
		tc.IgnoreTimes = true
		tc.ReconcileKey = fmt.Sprintf("%s/%s", testNS, sourceName)
		tc.Scheme = scheme.Scheme
		if tc.Reconciles == nil {
			tc.Reconciles = getSource()
		}

		c := tc.GetClient()
		dynClient := fake.NewSimpleDynamicClient(scheme.Scheme, tc.InitialState...)
		ctx := context.WithValue(context.Background(), dynamicclient.Key{}, dynClient)
		ctx, cancel := context.WithCancel(ctx)
		ctx = addressable.WithDuck(ctx)

		r := &reconciler{
			client:       c,
			scheme:       tc.Scheme,
			recorder:     recorder,
			sinkResolver: resolver.NewURIResolver(ctx, func(types.NamespacedName) {}),
		}
		if err := r.InjectClient(c); err != nil {
			t.Errorf("cannot inject client: %v", zap.Error(err))
		}

		t.Run(tc.Name, tc.Runner(t, r, c))
		cancel()
	}
}

func getSource() *sourcesv1alpha1.CamelSource {
	obj := &sourcesv1alpha1.CamelSource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: sourcesv1alpha1.SchemeGroupVersion.String(),
			Kind:       "CamelSource",
		},
		ObjectMeta: om(testNS, sourceName, generation),
		Spec: sourcesv1alpha1.CamelSourceSpec{
			Source: sourcesv1alpha1.CamelSourceOriginSpec{
				Flow: &sourcesv1alpha1.Flow{
					"from": map[string]interface{}{
						"uri": "timer:tick?period=3s",
					},
				},
			},
			Sink: &duckv1beta1.Destination{
				Ref: &corev1.ObjectReference{
					Name:       addressableName,
					Kind:       addressableKind,
					APIVersion: addressableAPIVersion,
				},
			},
		},
	}
	// selflink is not filled in when we create the object, so clear it
	obj.ObjectMeta.SelfLink = ""
	return obj
}

func withBumpedResourceVersion(cs *sourcesv1alpha1.CamelSource) *sourcesv1alpha1.CamelSource {
	cs.ResourceVersion = bumpVersion(cs.ResourceVersion)
	return cs
}

func bumpVersion(version string) string {
	v := 0
	if len(version) != 0 {
		v, _ = strconv.Atoi(version)
	}
	v++
	return fmt.Sprintf("%d", v)
}

func getCamelKSource() *sourcesv1alpha1.CamelSource {
	obj := &sourcesv1alpha1.CamelSource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: sourcesv1alpha1.SchemeGroupVersion.String(),
			Kind:       "CamelSource",
		},
		ObjectMeta: om(testNS, sourceName, generation),
		Spec: sourcesv1alpha1.CamelSourceSpec{
			Source: sourcesv1alpha1.CamelSourceOriginSpec{
				Integration: &camelv1.IntegrationSpec{
					Sources: []camelv1.SourceSpec{
						{
							DataSpec: camelv1.DataSpec{
								Name:    "integration.groovy",
								Content: "from('timer:tick?period=3s').to('knative://endpoint/sink')",
							},
						},
					},
				},
			},
			Sink: &duckv1beta1.Destination{
				Ref: &corev1.ObjectReference{
					Name:       addressableName,
					Kind:       addressableKind,
					APIVersion: addressableAPIVersion,
				},
			},
		},
	}
	// selflink is not filled in when we create the object, so clear it
	obj.ObjectMeta.SelfLink = ""
	return obj
}

func getCamelKFlowSource() *sourcesv1alpha1.CamelSource {
	flow := sourcesv1alpha1.Flow{
		"from": map[string]interface{}{
			"uri": "timer:tick?period=3s",
			"steps": []interface{}{
				map[string]interface{}{
					"set-body": map[string]interface{}{
						"constant": "Hello world",
					},
				},
			},
		},
	}
	obj := &sourcesv1alpha1.CamelSource{
		TypeMeta: metav1.TypeMeta{
			APIVersion: sourcesv1alpha1.SchemeGroupVersion.String(),
			Kind:       "CamelSource",
		},
		ObjectMeta: om(testNS, sourceName, generation),
		Spec: sourcesv1alpha1.CamelSourceSpec{
			Source: sourcesv1alpha1.CamelSourceOriginSpec{
				Flow: &flow,
			},
			Sink: &duckv1beta1.Destination{
				Ref: &corev1.ObjectReference{
					Name:       addressableName,
					Kind:       addressableKind,
					APIVersion: addressableAPIVersion,
				},
			},
		},
	}
	// selflink is not filled in when we create the object, so clear it
	obj.ObjectMeta.SelfLink = ""
	return obj
}

func withAlternativeContext(src *sourcesv1alpha1.CamelSource) *sourcesv1alpha1.CamelSource {
	if src.Spec.Source.Integration == nil {
		src.Spec.Source.Integration = &camelv1.IntegrationSpec{}
	}
	src.Spec.Source.Integration.Kit = alternativeKitName
	return src
}

func withBumpedResourceVersionIn(in *camelv1.Integration) *camelv1.Integration {
	in.ResourceVersion = bumpVersion(in.ResourceVersion)
	return in
}

func getContext() *camelv1.IntegrationKit {
	return makeContext(testNS, alternativeImage)
}

func getAlternativeContext() *camelv1.IntegrationKit {
	ctx := makeContext(testNS, alternativeImage)
	ctx.Name = alternativeKitName
	return ctx
}

func makeContext(namespace string, image string) *camelv1.IntegrationKit {
	ct := camelv1.IntegrationKit{
		TypeMeta: metav1.TypeMeta{
			Kind:       "IntegrationKit",
			APIVersion: "camel.apache.org/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "ctx-",
			Namespace:    namespace,
			Labels: map[string]string{
				"app":                       "camel-k",
				"camel.apache.org/kit.type": camelv1.IntegrationKitTypeExternal,
			},
		},
		Spec: camelv1.IntegrationKitSpec{
			Image:   image,
			Profile: camelv1.TraitProfileKnative,
		},
	}
	return &ct
}

func getRunningIntegration(t *testing.T) *camelv1.Integration {
	it, err := resources.MakeIntegration(&resources.CamelArguments{
		Name:      sourceName,
		Namespace: testNS,
		SinkURL:   addressableURI,
		Source: sourcesv1alpha1.CamelSourceOriginSpec{
			Flow: &sourcesv1alpha1.Flow{
				"from": map[string]interface{}{
					"uri": "timer:tick?period=3s",
				},
			},
		},
	})
	if err != nil {
		t.Error("failed to create integration", err)
	}
	it.ObjectMeta.OwnerReferences = getOwnerReferences()
	it.Status.Phase = camelv1.IntegrationPhaseRunning
	return it
}

func integrationWithAlternativeContext(integration *camelv1.Integration) *camelv1.Integration {
	integration.Spec.Kit = alternativeKitName
	return integration
}

func getRunningCamelKIntegration(t *testing.T) *camelv1.Integration {
	it, err := resources.MakeIntegration(&resources.CamelArguments{
		Name:      sourceName,
		Namespace: testNS,
		SinkURL:   addressableURI,
		Source: sourcesv1alpha1.CamelSourceOriginSpec{
			Integration: &camelv1.IntegrationSpec{
				Sources: []camelv1.SourceSpec{
					{
						DataSpec: camelv1.DataSpec{
							Name:    "integration.groovy",
							Content: "from('timer:tick?period=3s').to('knative://endpoint/sink')",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Error("failed to create integration", err)
	}
	it.ObjectMeta.OwnerReferences = getOwnerReferences()
	it.Status.Phase = camelv1.IntegrationPhaseRunning
	return it
}

func getRunningCamelKFlowIntegration(t *testing.T) *camelv1.Integration {
	it, err := resources.MakeIntegration(&resources.CamelArguments{
		Name:      sourceName,
		Namespace: testNS,
		SinkURL:   addressableURI,
		Source: sourcesv1alpha1.CamelSourceOriginSpec{
			Integration: &camelv1.IntegrationSpec{
				Sources: []camelv1.SourceSpec{
					{
						Loader: "knative-source",
						DataSpec: camelv1.DataSpec{
							Name:    "flow.yaml",
							Content: "- from:\n    steps:\n    - set-body:\n        constant: Hello world\n    uri: timer:tick?period=3s\n",
						},
					},
				},
			},
		},
	})
	if err != nil {
		t.Error("failed to create integration", err)
	}
	it.ObjectMeta.OwnerReferences = getOwnerReferences()
	it.Status.Phase = camelv1.IntegrationPhaseRunning
	return it
}

func getWrongIntegration(t *testing.T) *camelv1.Integration {
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

func withUpdatingIntegration(src *sourcesv1alpha1.CamelSource) *sourcesv1alpha1.CamelSource {
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

func asDeployedSource(src *sourcesv1alpha1.CamelSource) *sourcesv1alpha1.CamelSource {
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

func om(namespace, name string, generation int64) metav1.ObjectMeta {
	return metav1.ObjectMeta{
		Namespace:  namespace,
		Name:       name,
		Generation: generation,
		SelfLink:   fmt.Sprintf("/apis/eventing/sources/v1alpha1/namespaces/%s/object/%s", namespace, name),
		UID:        sourceUID,
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
					"url": addressableURI,
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
