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
	"os"

	sourcesv1alpha1 "github.com/knative/eventing-sources/pkg/apis/sources/v1alpha1"
	"github.com/knative/eventing-sources/pkg/reconciler/kuberneteseventsource/resources"
	"github.com/knative/pkg/logging"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	raImageEnvVar       = "K8S_RA_IMAGE"
	controllerAgentName = "kuberneteseventsource-controller"
)

// reconciler reconciles a KubernetesEventSource object
type reconciler struct {
	client.Client
	recorder            record.EventRecorder
	scheme              *runtime.Scheme
	receiveAdapterImage string
}

var _ reconcile.Reconciler = &reconciler{}

// Add creates a new KubernetesEventSource Controller and adds it to the Manager
// with default RBAC. The Manager will set fields on the Controller and Start it
// when the Manager is Started.
func Add(mgr manager.Manager) error {
	receiveAdapterImage, defined := os.LookupEnv(raImageEnvVar)
	if !defined {
		return fmt.Errorf("required environment variable %q not defined", raImageEnvVar)
	}

	return add(mgr, newReconciler(mgr, receiveAdapterImage))
}

func newReconciler(mgr manager.Manager, receiveAdapterImage string) reconcile.Reconciler {
	return &reconciler{
		Client:              mgr.GetClient(),
		scheme:              mgr.GetScheme(),
		recorder:            mgr.GetRecorder(controllerAgentName),
		receiveAdapterImage: receiveAdapterImage,
	}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New(controllerAgentName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to KubernetesEventSource
	err = c.Watch(&source.Kind{Type: &sourcesv1alpha1.KubernetesEventSource{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to owned ContainerSource
	err = c.Watch(&source.Kind{Type: &sourcesv1alpha1.ContainerSource{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &sourcesv1alpha1.KubernetesEventSource{},
	})
	if err != nil {
		return err
	}

	return nil
}

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
	logger := logging.FromContext(ctx).Named(controllerAgentName)
	ctx = logging.WithLogger(ctx, logger)
	logger.Debug("Reconciling", zap.String("name", source.Name), zap.String("namespace", source.Namespace))

	source.Status.InitializeConditions()

	//TODO How do we ensure this never creates two container sources due to cache latency?
	// Is this possible?
	// 1. User creates
	// 2. KES gets into cache
	// 3. Reconcile: create ContainerSource A, update KES status
	// 4. KES gets into cache
	// 5. Reconcile: create ContainerSource B, update KES status
	// 6. KES gets into cache
	// 7. ContainerSource A gets into cache
	// 8. ContainerSource B gets into cache
	// 9. Reconcile: which ContainerSource?
	cs, err := r.getOwnedContainerSource(ctx, source)
	if err != nil {
		if errors.IsNotFound(err) {
			cs := resources.MakeContainerSource(source, r.receiveAdapterImage)
			if err := controllerutil.SetControllerReference(source, cs, r.scheme); err != nil {
				return err
			}
			if err := r.Create(ctx, cs); err != nil {
				return err
			}
			r.recorder.Eventf(source, corev1.EventTypeNormal, "ContainerSourceCreated", "Created ContainerSource %q", cs.Name)
			// Wait for the ContainerSource to get a status
			return nil
		}
	}

	// Update ContainerSource spec if it's changed
	expected := resources.MakeContainerSource(source, r.receiveAdapterImage)
	if !equality.Semantic.DeepEqual(cs.Spec, expected.Spec) {
		cs.Spec = expected.Spec
		if r.Update(ctx, cs); err != nil {
			return err
		}
	}

	// Copy ContainerSource conditions to source
	source.Status.Conditions = cs.Status.Conditions.DeepCopy()
	return nil
}

func (r *reconciler) getOwnedContainerSource(ctx context.Context, source *sourcesv1alpha1.KubernetesEventSource) (*sourcesv1alpha1.ContainerSource, error) {
	list := &sourcesv1alpha1.ContainerSourceList{}
	err := r.List(ctx, &client.ListOptions{
		Namespace:     source.Namespace,
		LabelSelector: labels.Everything(),
		// TODO this is here because the fake client needs it.
		// Remove this when it's no longer needed.
		Raw: &metav1.ListOptions{
			TypeMeta: metav1.TypeMeta{
				APIVersion: sourcesv1alpha1.SchemeGroupVersion.String(),
				Kind:       "ContainerSource",
			},
		},
	},
		list)
	if err != nil {
		return nil, err
	}
	for _, cs := range list.Items {
		if metav1.IsControlledBy(&cs, source) {
			//TODO if there are >1 controlled, delete all but first?
			return &cs, nil
		}
	}
	return nil, errors.NewNotFound(sourcesv1alpha1.Resource("containersources"), "")
}

func (r *reconciler) update(ctx context.Context, u *sourcesv1alpha1.KubernetesEventSource) error {
	current := &sourcesv1alpha1.KubernetesEventSource{}
	err := r.Get(ctx, client.ObjectKey{Namespace: u.Namespace, Name: u.Name}, current)
	if err != nil {
		return err
	}

	if !equality.Semantic.DeepEqual(current.Status, u.Status) {
		current.Status = u.Status
		return r.Status().Update(ctx, current)
	}

	return nil
}
