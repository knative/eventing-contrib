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
	"context"

	"go.uber.org/zap"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/resolver"

	v1alpha1 "knative.dev/eventing-contrib/awssqs/pkg/apis/sources/v1alpha1"
	awssqssource "knative.dev/eventing-contrib/awssqs/pkg/client/injection/reconciler/sources/v1alpha1/awssqssource"
	"knative.dev/eventing-contrib/awssqs/pkg/reconciler/resources"
	eventingclientset "knative.dev/eventing/pkg/client/clientset/versioned"
	eventinglisters "knative.dev/eventing/pkg/client/listers/eventing/v1alpha1"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	pkgreconciler "knative.dev/pkg/reconciler"
)

// newReconciledNormal makes a new reconciler event with event type Normal, and
// reason AwsSqsSourceReconciled.
func newReconciledNormal(namespace, name string) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, "AwsSqsSourceReconciled", "AwsSqsSource reconciled: \"%s/%s\"", namespace, name)
}

// Reconciler implements controller.Reconciler for AwsSqsSource resources.
type Reconciler struct {
	receiveAdapterImage string

	kubeClientSet     kubernetes.Interface
	eventingClientSet eventingclientset.Interface
	eventTypeLister   eventinglisters.EventTypeLister
	sinkResolver      *resolver.URIResolver
}

// Check that our Reconciler implements Interface
var _ awssqssource.Interface = (*Reconciler)(nil)

// ReconcileKind implements Interface.ReconcileKind.
func (r *Reconciler) ReconcileKind(ctx context.Context, src *v1alpha1.AwsSqsSource) pkgreconciler.Event {
	src.Status.InitializeConditions()
	src.Status.ObservedGeneration = src.Generation

	// This Source attempts to reconcile three things.
	// 1. Determine the sink's URI.
	//     - Nothing to delete.
	// 2. Create a receive adapter in the form of a Deployment.
	//     - Will be garbage collected by K8s when this AwsSqsSource is deleted.
	// 3. Register that receive adapter as a Pull endpoint for the specified AWS SQS queue.

	if src.Spec.Sink != nil {
		dest := &duckv1.Destination{
			Ref: &duckv1.KReference{
				Kind:       src.Spec.Sink.Kind,
				Namespace:  src.Namespace,
				Name:       src.Spec.Sink.Name,
				APIVersion: src.Spec.Sink.APIVersion,
			},
		}
		sinkURI, err := r.sinkResolver.URIFromDestinationV1(*dest, src)
		if err != nil {
			src.Status.MarkNoSink("NotFound", "")
			return err
		}
		src.Status.MarkSink(sinkURI.String())
	} else {
		src.Status.MarkNoSink("NotFound", "")
		return nil
	}

	if _, err := r.createReceiveAdapter(ctx, src); err != nil {
		logging.FromContext(ctx).Error("Unable to create the receive adapter", zap.Error(err))
		return err
	}
	src.Status.MarkDeployed()

	return newReconciledNormal(src.Namespace, src.Name)
}

func (r *Reconciler) createReceiveAdapter(ctx context.Context, src *v1alpha1.AwsSqsSource) (*appsv1.Deployment, error) {
	ra, err := r.getReceiveAdapter(ctx, src)
	if err != nil && !apierrors.IsNotFound(err) {
		logging.FromContext(ctx).Error("Unable to get an existing receive adapter", zap.Error(err))
		return nil, err
	}

	if ra != nil {
		logging.FromContext(ctx).Desugar().Info("Reusing existing receive adapter", zap.Any("receiveAdapter", ra))
		// TODO: this is a bug, the deployment should be updated.
		return ra, nil
	}

	expected := resources.MakeReceiveAdapter(&resources.ReceiveAdapterArgs{
		Image:   r.receiveAdapterImage,
		Source:  src,
		Labels:  getLabels(src),
		SinkURI: src.Status.SinkURI.String(),
	})

	ra, err = r.kubeClientSet.AppsV1().Deployments(src.Namespace).Create(expected)
	logging.FromContext(ctx).Desugar().Info("Receive Adapter created.", zap.Error(err), zap.Any("receiveAdapter", ra))
	return ra, err
}

func (r *Reconciler) getReceiveAdapter(ctx context.Context, src *v1alpha1.AwsSqsSource) (*appsv1.Deployment, error) {
	dl, err := r.kubeClientSet.AppsV1().Deployments(src.Namespace).List(metav1.ListOptions{
		LabelSelector: r.getLabelSelector(src).String(),
	})

	if err != nil {
		logging.FromContext(ctx).Errorw("Unable to list deployments: %v", zap.Error(err))
		return nil, err
	}
	for _, dep := range dl.Items {
		if metav1.IsControlledBy(&dep, src) {
			return &dep, nil
		}
	}
	return nil, apierrors.NewNotFound(schema.GroupResource{}, "")
}

func (r *Reconciler) getLabelSelector(src *v1alpha1.AwsSqsSource) labels.Selector {
	return labels.SelectorFromSet(getLabels(src))
}

func getLabels(src *v1alpha1.AwsSqsSource) map[string]string {
	return map[string]string{
		"knative-eventing-source":      "awssqssource.sources.eventing.knative.dev",
		"knative-eventing-source-name": src.Name,
	}
}
