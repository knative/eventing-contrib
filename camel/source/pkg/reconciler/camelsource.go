/*
Copyright 2019 The Knative Authors

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

package reconciler

import (
	"context"
	"fmt"
	"log"
	"strings"

	camelv1alpha1 "github.com/apache/camel-k/pkg/apis/camel/v1alpha1"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/tools/record"
	"knative.dev/eventing-contrib/camel/source/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing-contrib/camel/source/pkg/reconciler/resources"
	"knative.dev/eventing-contrib/pkg/controller/sdk"
	"knative.dev/pkg/client/injection/ducks/duck/v1/addressable"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/resolver"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

const (
	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "camel-source-controller"
)

// Add creates a new CamelSource Controller and adds it to the Manager with
// default RBAC. The Manager will set fields on the Controller and Start it when
// the Manager is Started.
func Add(mgr manager.Manager, logger *zap.SugaredLogger) error {
	log.Println("Adding the Camel Source controller.")

	// Register Camel specific types
	mgr.GetScheme().AddKnownTypes(camelv1alpha1.SchemeGroupVersion, &camelv1alpha1.Integration{}, &camelv1alpha1.IntegrationList{})
	mgr.GetScheme().AddKnownTypes(camelv1alpha1.SchemeGroupVersion, &camelv1alpha1.IntegrationKit{}, &camelv1alpha1.IntegrationKitList{})
	mgr.GetScheme().AddKnownTypes(camelv1alpha1.SchemeGroupVersion, &camelv1alpha1.IntegrationPlatform{}, &camelv1alpha1.IntegrationPlatformList{})
	metav1.AddToGroupVersion(mgr.GetScheme(), camelv1alpha1.SchemeGroupVersion)

	ctx := context.Background()
	dynamicClient, err := dynamic.NewForConfig(mgr.GetConfig())
	if err != nil {
		return err
	}
	ctx = context.WithValue(ctx, dynamicclient.Key{}, dynamicClient)
	ctx = addressable.WithDuck(ctx)

	p := &sdk.Provider{
		AgentName: controllerAgentName,
		Parent:    &v1alpha1.CamelSource{},
		Owns:      []runtime.Object{&camelv1alpha1.Integration{}},
		Reconciler: &reconciler{
			recorder:     mgr.GetEventRecorderFor(controllerAgentName),
			scheme:       mgr.GetScheme(),
			sinkResolver: resolver.NewURIResolver(ctx, func(types.NamespacedName) {}),
		},
	}

	err = p.Add(mgr, logger)
	if err != nil {
		log.Println("Camel K cluster resources not installed correctly. Follow installation instructions at: https://github.com/apache/camel-k")
	}
	return err
}

type reconciler struct {
	client       client.Client
	scheme       *runtime.Scheme
	recorder     record.EventRecorder
	sinkResolver *resolver.URIResolver
}

// A CamelSource delegates the task of starting up the required containers to a Camel K Integration resource, that
// is managed by the Camel K operator (https://github.com/apache/camel-k).

// The Camel K operator is installed with Camel sources and requires a namespace-scoped IntegrationPlatform resource
// containing the global configuration. An IntegrationPlatform with a default configuration suitable for Knative is
// created by the reconcile loop if not present in the namespace of the CamelSource.

// When the CamelSource declares a specific image as starting point, the image is imported by the reconcile loop into an
// IntegrationContext. An IntegrationContext is a Camel K specific resource containing references to images and
// Camel-specific configuration.
// When no specific image is requested by the user, the Camel K operator will figure out how to construct an IntegrationContext.

// Reconcile compares the actual state with the desired, and attempts to converge the two.
func (r *reconciler) Reconcile(ctx context.Context, object runtime.Object) error {
	logger := logging.FromContext(ctx)

	source, ok := object.(*v1alpha1.CamelSource)
	if !ok {
		logger.Errorf("could not find camel source %v", object)
		return nil
	}

	// No need to reconcile if the source has been marked for deletion.
	deletionTimestamp := source.GetDeletionTimestamp()
	if deletionTimestamp != nil {
		return nil
	}

	source.Status.ObservedGeneration = source.Generation
	source.Status.InitializeConditions()

	if source.Spec.Sink == nil {
		source.Status.MarkNoSink("SinkMissing", "")
		return fmt.Errorf("spec.sink missing")
	}

	dest := source.Spec.Sink.DeepCopy()
	// fill optional data in destination
	if dest.Ref != nil {
		if dest.Ref.Namespace == "" {
			dest.Ref.Namespace = source.GetNamespace()
		}
	} else if dest.DeprecatedName != "" && dest.DeprecatedNamespace == "" {
		dest.DeprecatedNamespace = source.GetNamespace()
	}

	sinkURI, err := r.sinkResolver.URIFromDestination(*dest, source)
	if err != nil {
		source.Status.MarkNoSink("NotFound", "")
		return err
	}

	if dest.DeprecatedAPIVersion != "" &&
		dest.DeprecatedKind != "" &&
		dest.DeprecatedName != "" {
		source.Status.MarkSinkWarnRefDeprecated(sinkURI)
	} else {
		source.Status.MarkSink(sinkURI)
	}

	integration, err := r.reconcileIntegration(ctx, source, sinkURI)
	if err != nil {
		return err
	}

	// Update source status
	if integration != nil && integration.Status.Phase == camelv1alpha1.IntegrationPhaseRunning {
		source.Status.MarkDeployed()
	}

	return nil
}

func (r *reconciler) reconcileIntegration(ctx context.Context, source *v1alpha1.CamelSource, sinkURI string) (*camelv1alpha1.Integration, error) {
	logger := logging.FromContext(ctx)
	args := &resources.CamelArguments{
		Name:      source.Name,
		Namespace: source.Namespace,
		Source:    source.Spec.Source,
		SinkURL:   sinkURI,
	}
	if source.Spec.CloudEventOverrides != nil {
		args.Overrides = make(map[string]string)
		for k, v := range source.Spec.CloudEventOverrides.Extensions {
			args.Overrides[strings.ToLower(k)] = v
		}
	}

	integration, err := r.getIntegration(ctx, source)
	if err != nil {
		if k8serrors.IsNotFound(err) {
			integration, err = r.createIntegration(ctx, source, args)
			if err != nil {
				r.recorder.Eventf(source, corev1.EventTypeWarning, "IntegrationBlocked", "waiting for %v", err)
				return nil, err
			}
			r.recorder.Eventf(source, corev1.EventTypeNormal, "Deployed", "Created integration %q", integration.Name)
			source.Status.MarkDeploying("Deploying", "Created integration %s", integration.Name)
			// Since the Deployment has just been created, there's nothing more
			// to do until it gets a status. This CamelSource will be reconciled
			// again when the Integration is updated.
			return integration, nil
		}
		return nil, err
	}

	// Update Integration spec if it's changed
	expected, err := resources.MakeIntegration(args)
	if err != nil {
		return nil, err
	}
	// Since the Integration spec has fields defaulted by the webhook, it won't
	// be equal to expected. Use DeepDerivative to compare only the fields that
	// are set in expected.
	if !equality.Semantic.DeepDerivative(expected.Spec, integration.Spec) {
		logger.Infof("Integration %q in namespace %q has changed and needs to be updated", integration.Name, integration.Namespace)
		integration.Spec = expected.Spec
		err := r.client.Update(ctx, integration)
		// if no error, update the status.
		if err == nil {
			r.recorder.Eventf(source, corev1.EventTypeNormal, "Deployed", "Updated integration %q", integration.Name)
			source.Status.MarkDeploying("IntegrationUpdated", "Updated integration %s", integration.Name)
		} else {
			source.Status.MarkDeploying("IntegrationNeedsUpdate", "Attempting to update integration %s", integration.Name)
			r.recorder.Eventf(source, corev1.EventTypeWarning, "IntegrationNeedsUpdate", "Failed to update integration %q", integration.Name)
		}
		// Return after this update or error and reconcile again
		return nil, err
	}
	return integration, nil
}

func (r *reconciler) getIntegration(ctx context.Context, source *v1alpha1.CamelSource) (*camelv1alpha1.Integration, error) {
	logger := logging.FromContext(ctx)

	list := &camelv1alpha1.IntegrationList{}
	lo := &client.ListOptions{
		Namespace:     source.Namespace,
		LabelSelector: labels.Everything(),
		// TODO this is here because the fake client needs it.
		// Remove this when it's no longer needed.
		Raw: &metav1.ListOptions{
			TypeMeta: metav1.TypeMeta{
				APIVersion: camelv1alpha1.SchemeGroupVersion.String(),
				Kind:       "Integration",
			},
		},
	}
	err := r.client.List(ctx, list, lo)
	if err != nil {
		logger.Errorw("Unable to list integrations", zap.Error(err))
		return nil, err
	}
	for _, c := range list.Items {
		if metav1.IsControlledBy(&c, source) {
			return &c, nil
		}
	}
	return nil, k8serrors.NewNotFound(schema.GroupResource{}, "")
}

func (r *reconciler) createIntegration(ctx context.Context, source *v1alpha1.CamelSource, args *resources.CamelArguments) (*camelv1alpha1.Integration, error) {
	integration, err := resources.MakeIntegration(args)
	if err != nil {
		return nil, err
	}

	if err := controllerutil.SetControllerReference(source, integration, r.scheme); err != nil {
		return nil, err
	}

	if err := r.client.Create(ctx, integration); err != nil {
		return nil, err
	}
	return integration, nil
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}
