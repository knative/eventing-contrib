/*
Copyright 2020 The Knative Authors

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
	context "context"
	"fmt"
	"strings"

	camelv1 "github.com/apache/camel-k/pkg/apis/camel/v1"
	camelclientset "github.com/apache/camel-k/pkg/client/clientset/versioned"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	v1alpha1 "knative.dev/eventing-contrib/camel/source/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing-contrib/camel/source/pkg/reconciler/resources"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"

	camelsource "knative.dev/eventing-contrib/camel/source/pkg/client/injection/reconciler/sources/v1alpha1/camelsource"
)

// newReconciledNormal makes a new reconciler event with event type Normal, and
// reason CamelSourceReconciled.
func newReconciledNormal(namespace, name string) reconciler.Event {
	return reconciler.NewEvent(corev1.EventTypeNormal, "CamelSourceReconciled", "CamelSource reconciled: \"%s/%s\"", namespace, name)
}

// Reconciler implements controller.Reconciler for CamelSource resources.
type Reconciler struct {
	sinkResolver   *resolver.URIResolver
	camelClientSet camelclientset.Interface
}

// Check that our Reconciler implements Interface
var _ camelsource.Interface = (*Reconciler)(nil)

// ReconcileKind implements Interface.ReconcileKind.
func (r *Reconciler) ReconcileKind(ctx context.Context, source *v1alpha1.CamelSource) reconciler.Event {
	source.Status.InitializeConditions()
	source.Status.ObservedGeneration = source.Generation

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

	// Reconcile and update integration status
	if integration, err := r.reconcileIntegration(ctx, source, sinkURI); err != nil {
		return err
	} else if integration != nil && integration.Status.Phase == camelv1.IntegrationPhaseRunning {
		source.Status.MarkDeployed()
	}

	return newReconciledNormal(source.Namespace, source.Name)
}

func (r *Reconciler) reconcileIntegration(ctx context.Context, source *v1alpha1.CamelSource, sinkURI string) (*camelv1.Integration, reconciler.Event) {
	logger := logging.FromContext(ctx)
	args := &resources.CamelArguments{
		Name:      source.Name,
		Namespace: source.Namespace,
		Source:    source.Spec.Source,
		Owner:     source,
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
				return nil, reconciler.NewEvent(corev1.EventTypeWarning, "IntegrationBlocked", "waiting for %v", err)
			}

			// Since the Deployment has just been created, there's nothing more
			// to do until it gets a status. This CamelSource will be reconciled
			// again when the Integration is updated.
			name := integration.Name
			if name == "" {
				name = fmt.Sprintf("%s*", integration.GenerateName)
			}
			source.Status.MarkDeploying("Deploying", "Created integration %s", name)
			return integration, reconciler.NewEvent(corev1.EventTypeNormal, "Deployed", "Created integration %q", name)
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

		_, err := r.updateIntegration(ctx, integration)
		// if no error, update the status.
		name := integration.Name
		if name == "" {
			name = fmt.Sprintf("%s*", integration.GenerateName)
		}
		if err == nil {
			source.Status.MarkDeploying("IntegrationUpdated", "Updated integration %s", name)
			return nil, reconciler.NewEvent(corev1.EventTypeNormal, "IntegrationUpdated", "Updated integration %q", name)
		} else {
			source.Status.MarkDeploying("IntegrationNeedsUpdate", "Attempting to update integration %s", name)
			return nil, reconciler.NewEvent(corev1.EventTypeWarning, "IntegrationNeedsUpdate", "Failed to update integration %q", name)
		}
	}
	return integration, nil
}

func (r *Reconciler) getIntegration(ctx context.Context, source *v1alpha1.CamelSource) (*camelv1.Integration, error) {
	all, err := r.camelClientSet.CamelV1().Integrations(source.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, a := range all.Items {
		if metav1.IsControlledBy(&a, source) {
			return &a, nil
		}
	}
	return nil, k8serrors.NewNotFound(schema.GroupResource{}, "")
}

func (r *Reconciler) createIntegration(ctx context.Context, source *v1alpha1.CamelSource, args *resources.CamelArguments) (*camelv1.Integration, error) {
	integration, err := resources.MakeIntegration(args)
	if err != nil {
		return nil, err
	}

	return r.camelClientSet.CamelV1().Integrations(source.Namespace).Create(integration)
}

func (r *Reconciler) updateIntegration(ctx context.Context, integration *camelv1.Integration) (*camelv1.Integration, error) {
	return r.camelClientSet.CamelV1().Integrations(integration.Namespace).Update(integration)
}
