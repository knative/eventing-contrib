/*
Copyright 2018 The Knative Authors

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

package kuberneteseventsource

import (
	"context"
	"fmt"

	sourcesv1alpha1 "github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	"github.com/knative/pkg/apis/duck"
	duckv1alpha1 "github.com/knative/pkg/apis/duck/v1alpha1"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

// TODO(n3wscott): To show the source is working, and while knative eventing is
// defining the ducktypes, tryTargetable allows us to point to a Service.service
// to validate the source is working.
const tryTargetable = true

// Reconcile reads the state of the cluster for a KubernetesEventSource object
// and makes changes based on the state read and what is in the
// KubernetesEventSource.Spec.
func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.Background()
	// TODO use controller-runtime logf to get a scoped logger for this controller
	logger := logging.FromContext(ctx)
	logger.Debug("Reconciling", zap.Any("key", request.NamespacedName))

	// Fetch the KubernetesEventSource resource
	instance := &sourcesv1alpha1.KubernetesEventSource{}
	err := r.Get(ctx, request.NamespacedName, instance)
	if err != nil {
		// If the resource cannot be found, it was probably deleted since being
		// added to the workqueue. Return a successful result without retrying.
		if errors.IsNotFound(err) {
			logger.Warn("Could not find KubernetesEventSource", zap.Any("key", request.NamespacedName))
			return reconcile.Result{}, nil
		}
		// If the resource exists but cannot be retrieved, then retry.
		return reconcile.Result{}, err
	}

	// The resource exists, so we can reconcile it. An error returned
	// here means the reconcile did not complete and the resource should be
	// requeued for another attempt.
	// A successful reconcile does not mean the resource is in the desired state,
	// it means no more can be done for now. The resource will not be reconciled
	// again until the resync period expires or a watched resource changes.
	var reconcileErr error
	if reconcileErr = r.reconcile(ctx, instance); reconcileErr != nil {
		logger.Error("Reconcile error", zap.Error(reconcileErr))
	}

	// Since the reconcile is a sequence of steps, earlier steps may complete
	// successfully while later steps fail. The resource is updated on failure to
	// preserve any useful status or metadata mutations made before the failure
	// occurred.
	if updateErr := r.update(ctx, instance); updateErr != nil {
		// An error here means the instance should be reconciled again, regardless
		// of whether the reconcile was successful or not.
		return reconcile.Result{}, updateErr
	}

	return reconcile.Result{}, reconcileErr
}

func (r *reconciler) reconcile(ctx context.Context, source *sourcesv1alpha1.KubernetesEventSource) error {
	logger := logging.FromContext(ctx)

	source.Status.InitializeConditions()

	uri, err := r.getSinkURI(ctx, source)
	if err != nil {
		source.Status.MarkNoSink("NotFound", "")
		return err
	}
	source.Status.MarkSink(uri)

	return nil
}

func (r *reconciler) getSinkURI(ctx context.Context, source *sourcesv1alpha1.KubernetesEventSource) (string, error) {
	// check to see if the source has provided a sink ref in the spec. Lets look for it.
	if source.Spec.Sink == nil {
		return "", fmt.Errorf("sink ref is nil")
	}

	obj, err := r.fetchObjectReference(ctx, source.Namespace, source.Spec.Sink)
	if err != nil {
		return "", err
	}
	t := duckv1alpha1.Sink{}
	err = duck.FromUnstructured(obj, &t)
	if err != nil {
		// logger.Warnf("Failed to deserialize sink: %s", zap.Error(err))
		return "", err
	}

	if t.Status.Sinkable != nil {
		return fmt.Sprintf("http://%s/", t.Status.Sinkable.DomainInternal), nil
	}

	// for now we will try again as a targetable.
	if tryTargetable {
		t := duckv1alpha1.Target{}
		err = duck.FromUnstructured(obj, &t)
		if err != nil {
			// logger.Warnf("Failed to deserialize targetable: %s", zap.Error(err))
			return "", err
		}

		if t.Status.Targetable != nil {
			return fmt.Sprintf("http://%s/", t.Status.Targetable.DomainInternal), nil
		}
	}

	return "", fmt.Errorf("sink does not contain sinkable")
}

// fetchObjectReference fetches an object based on ObjectReference.
//TODO(grantr) use controller-runtime unstructured client
func (r *reconciler) fetchObjectReference(ctx context.Context, namespace string, ref *corev1.ObjectReference) (duck.Marshalable, error) {
	logger := logging.FromContext(ctx)

	resourceClient, err := r.CreateResourceInterface(namespace, ref)
	if err != nil {
		logger.Warnf("failed to create dynamic client resource: %v", zap.Error(err))
		return nil, err
	}

	return resourceClient.Get(ref.Name, metav1.GetOptions{})
}

func (r *reconciler) CreateResourceInterface(namespace string, ref *corev1.ObjectReference) (dynamic.ResourceInterface, error) {
	rc := r.dynamicClient.Resource(duck.KindToResource(ref.GroupVersionKind()))
	if rc == nil {
		return nil, fmt.Errorf("failed to create dynamic client resource")
	}
	return rc.Namespace(namespace), nil

}

func (r *reconciler) update(ctx context.Context, u *sourcesv1alpha1.KubernetesEventSource) error {
	current := &sourcesv1alpha1.KubernetesEventSource{}
	err := r.Get(ctx, client.ObjectKey{Namespace: u.Namespace, Name: u.Name}, current)

	if err != nil {
		return err
	}

	updated := false
	if !equality.Semantic.DeepEqual(current.OwnerReferences, u.OwnerReferences) {
		current.OwnerReferences = u.OwnerReferences
		updated = true
	}

	if !equality.Semantic.DeepEqual(current.Finalizers, u.Finalizers) {
		current.Finalizers = u.Finalizers
		updated = true
	}

	if !equality.Semantic.DeepEqual(current.Status, u.Status) {
		current.Status = u.Status
		updated = true
	}

	if updated == false {
		return nil
	}
	// Until #38113 is merged, we must use Update instead of UpdateStatus to
	// update the Status block of the Feed resource. UpdateStatus will not
	// allow changes to the Spec of the resource, which is ideal for ensuring
	// nothing other than resource status has been updated.
	return r.Update(ctx, current)
}
