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
	"testing"

	clientgotesting "k8s.io/client-go/testing"
	fakesourceclient "knative.dev/eventing-contrib/camel/source/pkg/client/injection/client/fake"
	"knative.dev/eventing-contrib/camel/source/pkg/client/injection/reconciler/sources/v1alpha1/camelsource"
	reconcilertesting "knative.dev/eventing-contrib/camel/source/pkg/reconciler/testing"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	logtesting "knative.dev/pkg/logging/testing"

	camelv1 "github.com/apache/camel-k/pkg/apis/camel/v1"
	"go.uber.org/zap"
	v1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	sourcesv1alpha1 "knative.dev/eventing-contrib/camel/source/pkg/apis/sources/v1alpha1"
	fakecamelclient "knative.dev/eventing-contrib/camel/source/pkg/camel-k/injection/client/fake"
	"knative.dev/eventing-contrib/camel/source/pkg/reconciler/resources"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	duckv1alpha1 "knative.dev/pkg/apis/duck/v1alpha1"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	"knative.dev/pkg/resolver"

	. "knative.dev/eventing-contrib/camel/source/pkg/reconciler/testing"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	alternativeImage   = "apache/camel-k-base-alternative"
	alternativeKitName = "alternative-kit"

	sourceName = "test-camel-source"
	testNS     = "testnamespace"
	generation = 1

	addressableName       = "testsink"
	addressableKind       = "Addressable"
	addressableAPIVersion = "duck.knative.dev/v1"
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
	key := testNS + "/" + sourceName
	sink := &corev1.ObjectReference{
		Kind:       addressableKind,
		Namespace:  testNS,
		Name:       addressableName,
		APIVersion: addressableAPIVersion,
	}

	table := TableTest{{
		Name: "bad workqueue key",
		// Make sure Reconcile handles bad keys.
		Key: "too/many/parts",
	}, {
		Name: "key not found",
		// Make sure Reconcile handles good keys that don't exist.
		Key: "not-found",
	}, {
		Name: "Deleted source",
		Key:  key,
		Objects: []runtime.Object{
			NewCamelSource(sourceName, testNS, generation,
				WithCamelSourceDeleted),
		},
	}, {
		Name: "Cannot get sink URI",
		Key:  key,
		Objects: []runtime.Object{
			getBaseSource(),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: getBaseSource(
				// Updates
				WithInitCamelSource,
				WithCamelSourceSinkNotFound()),
		}},
		WantErr: true,
		WantEvents: []string{
			Eventf(corev1.EventTypeWarning, "InternalError", `failed to get ref %s: addressables.duck.knative.dev "testsink" not found`, sink),
		},
	}, {
		Name: "Creating integration",
		Key:  key,
		Objects: []runtime.Object{
			getBaseSource(),
			getAddressable(),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: getBaseSource(
				// Updates
				WithInitCamelSource,
				WithCamelSourceSink(addressableURI),
				WithCamelSourceDeploying()),
		}},
		WantCreates: []runtime.Object{
			NewIntegration("test-camel-source", testNS, getBaseSource(WithCamelSourceSink(addressableURI))),
		},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "Deployed", `Created integration "test-camel-source-*"`),
		},
	}, {
		Name: "Source Deployed",
		Key:  key,
		Objects: []runtime.Object{
			getBaseSource(
				WithInitCamelSource,
				WithCamelSourceSink(addressableURI),
				WithCamelSourceDeploying()),
			getAddressable(),
			getRunningIntegration(),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: getBaseSource(
				WithInitCamelSource,
				WithCamelSourceSink(addressableURI),
				WithCamelSourceDeploying(),
				// Updates
				WithCamelSourceDeployed(),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "CamelSourceReconciled", `CamelSource reconciled: "testnamespace/test-camel-source"`),
		},
	}, {
		Name: "Camel K Source Deployed",
		Key:  key,
		Objects: []runtime.Object{
			getCamelKSource(),
			getAddressable(),
			getRunningCamelKIntegration(t),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: getCamelKSource(
				// Updates
				WithCamelSourceDeployed(),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "CamelSourceReconciled", `CamelSource reconciled: "testnamespace/test-camel-source"`),
		},
	}, {
		Name: "Source changed",
		Key:  key,
		Objects: []runtime.Object{
			getBaseSource(),
			getAddressable(),
			getWrongIntegration(),
		},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: getRunningIntegration(),
		}},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: getBaseSource(
				// Updates
				WithInitCamelSource,
				WithCamelSourceSink(addressableURI),
				WithCamelSourceIntegrationUpdated()),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "IntegrationUpdated", `Updated integration "test-camel-source-*"`),
		},
	}, {
		Name: "Camel K Source changed",
		Key:  key,
		Objects: []runtime.Object{
			getCamelKSource(),
			getAddressable(),
			getWrongIntegration(),
		},
		WantUpdates: []clientgotesting.UpdateActionImpl{{
			Object: getRunningCamelKIntegration(t),
		}},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: getCamelKSource(WithCamelSourceIntegrationUpdated()),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "IntegrationUpdated", `Updated integration "test-camel-source-*"`),
		},
	}, {
		Name: "Source with context", Key: key,
		Objects: []runtime.Object{
			withAlternativeContext(getBaseSource(
				WithInitCamelSource,
				WithCamelSourceSink(addressableURI))),
			getAddressable(),
			integrationWithAlternativeContext(getRunningIntegration()),
			getAlternativeContext(),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: withAlternativeContext(getBaseSource(
				WithInitCamelSource,
				WithCamelSourceSink(addressableURI),
				// Updates
				WithCamelSourceDeployed(),
			)),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "CamelSourceReconciled", `CamelSource reconciled: "testnamespace/test-camel-source"`),
		},
	}, {
		Name: "Camel K Source with context",
		Key:  key,
		Objects: []runtime.Object{
			withAlternativeContext(getCamelKSource()),
			getAddressable(),
			integrationWithAlternativeContext(getRunningCamelKIntegration(t)),
			getAlternativeContext(),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: withAlternativeContext(getCamelKSource(
				// Updates
				WithCamelSourceDeployed(),
			)),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "CamelSourceReconciled", `CamelSource reconciled: "testnamespace/test-camel-source"`),
		},
	}, {
		Name: "Camel K Flow source",
		Key:  key,
		Objects: []runtime.Object{
			getCamelKFlowSource(),
			getAddressable(),
			getRunningCamelKFlowIntegration(t),
		},
		WantStatusUpdates: []clientgotesting.UpdateActionImpl{{
			Object: getCamelKFlowSource(
				// Updates
				WithCamelSourceDeployed(),
			),
		}},
		WantEvents: []string{
			Eventf(corev1.EventTypeNormal, "CamelSourceReconciled", `CamelSource reconciled: "testnamespace/test-camel-source"`),
		},
	}}

	defer logtesting.ClearAll()

	table.Test(t, reconcilertesting.MakeFactory(func(ctx context.Context, listers *Listers, cmw configmap.Watcher) controller.Reconciler {
		ctx = addressable.WithDuck(ctx)
		r := &Reconciler{
			camelClientSet: fakecamelclient.Get(ctx),
			sinkResolver:   resolver.NewURIResolver(ctx, func(types.NamespacedName) {}),
		}
		return camelsource.NewReconciler(ctx, logging.FromContext(ctx),
			fakesourceclient.Get(ctx), listers.GetCamelSourcesLister(),
			controller.GetEventRecorder(ctx), r)
	}, zap.L()))
}

func getCamelKSource(opts ...CamelSourceOption) *sourcesv1alpha1.CamelSource {
	opts = append([]CamelSourceOption{
		WithInitCamelSource,
		WithCamelSourceSpec(sourcesv1alpha1.CamelSourceSpec{
			Source: sourcesv1alpha1.CamelSourceOriginSpec{
				Integration: &camelv1.IntegrationSpec{
					Sources: []camelv1.SourceSpec{{
						DataSpec: camelv1.DataSpec{
							Name:    "integration.groovy",
							Content: "from('timer:tick?period=3s').to('knative://endpoint/sink')",
						},
					}},
				},
			},
			Sink: &duckv1beta1.Destination{
				Ref: &corev1.ObjectReference{
					Name:       addressableName,
					Kind:       addressableKind,
					APIVersion: addressableAPIVersion,
				},
			},
		}),
		WithCamelSourceSink(addressableURI),
		WithCamelSourceDeploying()}, opts...)

	return getBaseSource(opts...)
}

func getCamelKFlowSource(opts ...CamelSourceOption) *sourcesv1alpha1.CamelSource {
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

	opts = append([]CamelSourceOption{
		WithInitCamelSource,
		WithCamelSourceSpec(sourcesv1alpha1.CamelSourceSpec{
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
		}),
		WithCamelSourceSink(addressableURI),
		WithCamelSourceDeploying()}, opts...)

	return getBaseSource(opts...)
}

func withAlternativeContext(src *sourcesv1alpha1.CamelSource) *sourcesv1alpha1.CamelSource {
	if src.Spec.Source.Integration == nil {
		src.Spec.Source.Integration = &camelv1.IntegrationSpec{}
	}
	src.Spec.Source.Integration.Kit = alternativeKitName
	return src
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

func getRunningIntegration() *camelv1.Integration {
	it := NewIntegration(sourceName, testNS, getBaseSource(WithCamelSourceSink(addressableURI)))

	it.Status.Phase = camelv1.IntegrationPhaseRunning
	return it
}

func integrationWithAlternativeContext(integration *camelv1.Integration) *camelv1.Integration {
	integration.Spec.Kit = alternativeKitName
	return integration
}

func getRunningCamelKIntegration(t *testing.T) *camelv1.Integration {
	src := getCamelKSource()
	it, err := resources.MakeIntegration(&resources.CamelArguments{
		Name:      sourceName,
		Namespace: testNS,
		SinkURL:   addressableURI,
		Owner:     src,
		Source:    src.Spec.Source,
	})
	if err != nil {
		t.Error("failed to create integration", err)
	}
	it.Status.Phase = camelv1.IntegrationPhaseRunning
	return it
}

func getRunningCamelKFlowIntegration(t *testing.T) *camelv1.Integration {
	src := getCamelKFlowSource()
	it, err := resources.MakeIntegration(&resources.CamelArguments{
		Name:      sourceName,
		Namespace: testNS,
		SinkURL:   addressableURI,
		Owner:     src,
		Source:    src.Spec.Source,
	})
	if err != nil {
		t.Error("failed to create integration", err)
	}
	it.Status.Phase = camelv1.IntegrationPhaseRunning
	return it
}

func getWrongIntegration() *camelv1.Integration {
	it := getRunningIntegration()
	it.Spec.Sources[0].Content = "wrong"
	return it
}

func getBaseSource(opts ...CamelSourceOption) *sourcesv1alpha1.CamelSource {
	opts = append([]CamelSourceOption{
		WithCamelSourceSpec(sourcesv1alpha1.CamelSourceSpec{
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
		})}, opts...)

	return NewCamelSource(sourceName, testNS, generation,
		opts...)
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
