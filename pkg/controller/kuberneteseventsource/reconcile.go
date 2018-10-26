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

	sourcesv1alpha1 "github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	"github.com/knative/eventing-sources/pkg/controller/kuberneteseventsource/resources"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
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
	logger.Debug("Reconciling", zap.String("name", source.Name), zap.String("namespace", source.Namespace))

	source.Status.InitializeConditions()

	cs := &sourcesv1alpha1.ContainerSource{}
	//TODO is it necessary for the name to be the same? Can we search for a ContainerSource
	// owned by this KubernetesEventSource instead?
	csName := resources.ContainerSourceName(source)

	if err := r.Get(ctx, client.ObjectKey{Namespace: source.Namespace, Name: csName}, cs); err != nil {
		if errors.IsNotFound(err) {
			cs := resources.MakeContainerSource(source)
			if err := r.Create(ctx, cs); err != nil {
				return err
			}
			r.recorder.Eventf(source, corev1.EventTypeNormal, "ContainerSourceCreated", "Created ContainerSource %q", source.Name)
		}
	}

	// TODO Hoist statuses from ContainerSource

	return nil
}

func (r *reconciler) update(ctx context.Context, u *sourcesv1alpha1.KubernetesEventSource) error {
	current := &sourcesv1alpha1.KubernetesEventSource{}
	err := r.Get(ctx, client.ObjectKey{Namespace: u.Namespace, Name: u.Name}, current)
	if err != nil {
		return err
	}

	if !equality.Semantic.DeepEqual(current.Status, u.Status) {
		current.Status = u.Status
		// Until #38113 is merged, we must use Update instead of UpdateStatus to
		// update the Status block of the Feed resource. UpdateStatus will not
		// allow changes to the Spec of the resource, which is ideal for ensuring
		// nothing other than resource status has been updated.
		return r.Update(ctx, current)
	}

	return nil
}
