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
	"encoding/json"
	"fmt"
	"strings"

	camelv1 "github.com/apache/camel-k/pkg/apis/camel/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	k8serrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	v1alpha1 "knative.dev/eventing-contrib/camel/source/pkg/apis/sources/v1alpha1"
	camelsource "knative.dev/eventing-contrib/camel/source/pkg/client/injection/reconciler/sources/v1alpha1/camelsource"
	"knative.dev/eventing-contrib/camel/source/pkg/reconciler/resources"
	"knative.dev/pkg/apis/duck"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/reconciler"
	"knative.dev/pkg/resolver"
)

// newReconciledNormal makes a new reconciler event with event type Normal, and
// reason CamelSourceReconciled.
func newReconciledNormal(namespace, name string) reconciler.Event {
	return reconciler.NewEvent(corev1.EventTypeNormal, "CamelSourceReconciled", "CamelSource reconciled: \"%s/%s\"", namespace, name)
}

// Reconciler implements controller.Reconciler for CamelSource resources.
type Reconciler struct {
	sinkResolver     *resolver.URIResolver
	dynamicClientSet dynamic.Interface
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
			source.Status.MarkDeploying("Deploying", "Created integration %s", integration.Name)
			// Since the Deployment has just been created, there's nothing more
			// to do until it gets a status. This CamelSource will be reconciled
			// again when the Integration is updated.
			return integration, reconciler.NewEvent(corev1.EventTypeNormal, "Deployed", "Created integration %q", integration.Name)
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
		if err == nil {
			source.Status.MarkDeploying("IntegrationUpdated", "Updated integration %s", integration.Name)
			return nil, reconciler.NewEvent(corev1.EventTypeNormal, "Deployed", "Updated integration %q", integration.Name)
		} else {
			source.Status.MarkDeploying("IntegrationNeedsUpdate", "Attempting to update integration %s", integration.Name)
			return nil, reconciler.NewEvent(corev1.EventTypeWarning, "IntegrationNeedsUpdate", "Failed to update integration %q", integration.Name)
		}
	}
	return integration, nil
}

var (
	integrationsGVR = schema.GroupVersionResource{
		Group:    camelv1.SchemeGroupVersion.Group,
		Version:  camelv1.SchemeGroupVersion.Version,
		Resource: "integrations",
	}
)

func (r *Reconciler) getIntegration(ctx context.Context, source *v1alpha1.CamelSource) (*camelv1.Integration, error) {
	usl, err := r.dynamicClientSet.Resource(integrationsGVR).Namespace(source.Namespace).List(metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	for _, u := range usl.Items {
		if metav1.IsControlledBy(&u, source) {
			i := camelv1.Integration{}
			if err := duck.FromUnstructured(&u, &i); err != nil {
				return nil, err
			}
			return &i, nil
		}
	}
	return nil, k8serrors.NewNotFound(schema.GroupResource{}, "")
}

func (r *Reconciler) createIntegration(ctx context.Context, source *v1alpha1.CamelSource, args *resources.CamelArguments) (*camelv1.Integration, error) {
	integration, err := resources.MakeIntegration(args)
	if err != nil {
		return nil, err
	}

	// Convert desired to unstructured.Unstructured
	b, err := json.Marshal(integration)
	if err != nil {
		return nil, err
	}
	ud := &unstructured.Unstructured{}
	if err := json.Unmarshal(b, ud); err != nil {
		return nil, err
	}

	u, err := r.dynamicClientSet.Resource(integrationsGVR).Namespace(source.Namespace).Create(ud, metav1.CreateOptions{})
	if err != nil {
		return nil, err
	}
	i := camelv1.Integration{}
	if err := duck.FromUnstructured(u, &i); err != nil {
		return nil, err
	}
	return &i, nil
}

func (r *Reconciler) updateIntegration(ctx context.Context, integration *camelv1.Integration) (*camelv1.Integration, error) {
	// Convert desired to unstructured.Unstructured
	b, err := json.Marshal(integration)
	if err != nil {
		return nil, err
	}
	ud := &unstructured.Unstructured{}
	if err := json.Unmarshal(b, ud); err != nil {
		return nil, err
	}

	u, err := r.dynamicClientSet.Resource(integrationsGVR).Namespace(integration.Namespace).Update(ud, metav1.UpdateOptions{})
	if err != nil {
		return nil, err
	}
	i := camelv1.Integration{}
	if err := duck.FromUnstructured(u, &i); err != nil {
		return nil, err
	}
	return &i, nil
}
