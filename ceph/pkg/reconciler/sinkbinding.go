package reconciler

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/pkg/apis/sources/v1alpha2"
	eventingclient "knative.dev/eventing/pkg/client/clientset/versioned"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"
	"knative.dev/pkg/tracker"
	"knative.dev/eventing-contrib/ceph/pkg/reconciler/resources"
)

// newSinkBindingCreated makes a new reconciler event with event type Normal, and
// reason SinkBindingCreated.
func newSinkBindingCreated(namespace, name string) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, "SinkBindingCreated", "created SinkBinding: \"%s/%s\"", namespace, name)
}

// newSinkBindingFailed makes a new reconciler event with event type Warning, and
// reason SinkBindingFailed.
func newSinkBindingFailed(namespace, name string, err error) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeWarning, "SinkBindingFailed", "failed to create SinkBinding: \"%s/%s\", %w", namespace, name, err)
}

// newSinkBindingUpdated makes a new reconciler event with event type Normal, and
// reason SinkBindingUpdated.
func newSinkBindingUpdated(namespace, name string) pkgreconciler.Event {
	return pkgreconciler.NewEvent(corev1.EventTypeNormal, "SinkBindingUpdated", "updated SinkBinding: \"%s/%s\"", namespace, name)
}

type SinkBindingReconciler struct {
	EventingClientSet eventingclient.Interface
}

func (r *SinkBindingReconciler) ReconcileSinkBinding(ctx context.Context, owner kmeta.OwnerRefable, source duckv1.SourceSpec, subject tracker.Reference) (*v1alpha2.SinkBinding, pkgreconciler.Event) {
	expected := resources.MakeSinkBinding(owner, source, subject)

	namespace := owner.GetObjectMeta().GetNamespace()
	sb, err := r.EventingClientSet.SourcesV1alpha2().SinkBindings(namespace).Get(ctx, expected.Name, metav1.GetOptions{})
	if apierrors.IsNotFound(err) {
		sb, err = r.EventingClientSet.SourcesV1alpha2().SinkBindings(namespace).Create(ctx, expected, metav1.CreateOptions{})
		if err != nil {
			return nil, newSinkBindingFailed(expected.Namespace, expected.Name, err)
		}
		return sb, newSinkBindingCreated(sb.Namespace, sb.Name)
	} else if err != nil {
		return nil, fmt.Errorf("error getting SinkBinding %q: %v", expected.Name, err)
	} else if !metav1.IsControlledBy(sb, owner.GetObjectMeta()) {
		return nil, fmt.Errorf("SinkBinding %q is not owned by %s %q",
			sb.Name, owner.GetGroupVersionKind().Kind, owner.GetObjectMeta().GetName())
	} else if r.specChanged(sb.Spec, expected.Spec) {
		sb.Spec = expected.Spec
		if sb, err = r.EventingClientSet.SourcesV1alpha2().SinkBindings(namespace).Update(ctx, sb, metav1.UpdateOptions{}); err != nil {
			return sb, err
		}
		return sb, newSinkBindingUpdated(sb.Namespace, sb.Name)
	} else {
		logging.FromContext(ctx).Debugw("Reusing existing sink binding", zap.Any("sinkBinding", sb))
	}
	return sb, nil
}

func (r *SinkBindingReconciler) specChanged(oldSpec v1alpha2.SinkBindingSpec, newSpec v1alpha2.SinkBindingSpec) bool {
	return !equality.Semantic.DeepDerivative(newSpec, oldSpec)
}
