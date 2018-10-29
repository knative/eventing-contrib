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

package sdk

import (
	"context"

	"go.uber.org/zap"

	"github.com/knative/pkg/logging"
	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/record"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type Reconciler struct {
	client        client.Client
	dynamicClient dynamic.Interface
	recorder      record.EventRecorder
	scheme        *runtime.Scheme

	provider Provider
}

// Verify the struct implements reconcile.Reconciler
var _ reconcile.Reconciler = &Reconciler{}

// Reconcile compares the actual state with the desired, and attempts to
// converge the two.
func (r *Reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.TODO()
	logger := logging.FromContext(ctx)

	logger.Infof("Reconciling %s %v", r.provider.Parent.GetObjectKind(), request)

	obj := r.provider.Parent.DeepCopyObject()

	err := r.client.Get(context.TODO(), request.NamespacedName, obj)

	if errors.IsNotFound(err) {
		logger.Errorf("could not find %s %v\n", r.provider.Parent.GetObjectKind(), request)
		return reconcile.Result{}, nil
	}

	if err != nil {
		logger.Errorf("could not fetch %s %v for %+v\n", r.provider.Parent.GetObjectKind(), err, request)
		return reconcile.Result{}, err
	}

	original := obj.DeepCopyObject()

	// Reconcile this copy of the Source and then write back any status
	// updates regardless of whether the reconcile error out.
	obj, reconcileErr := r.provider.Reconciler.Reconcile(ctx, obj)
	if reconcileErr != nil {
		logger.Warnf("Failed to reconcile %s: %v", r.provider.Parent.GetObjectKind(), reconcileErr)
	}

	if needsUpdate := r.needsUpdate(ctx, original, obj); needsUpdate {
		if _, updateErr := r.update(ctx, request, obj); updateErr != nil {
			logger.Desugar().Error("Failed to update", zap.Error(err), zap.Any("objectKind", r.provider.Parent.GetObjectKind()))
			return reconcile.Result{}, updateErr
		}
	}

	// Requeue if the resource is not ready:
	return reconcile.Result{}, reconcileErr
}

func (r *Reconciler) InjectClient(c client.Client) error {
	r.client = c
	if r.provider.Reconciler != nil {
		r.provider.Reconciler.InjectClient(c)
	}
	return nil
}

func (r *Reconciler) InjectConfig(c *rest.Config) error {
	var err error
	r.dynamicClient, err = dynamic.NewForConfig(c)
	if r.provider.Reconciler != nil {
		r.provider.Reconciler.InjectConfig(c)
	}
	return err
}

func (r *Reconciler) needsUpdate(ctx context.Context, old, new runtime.Object) bool {
	if old == nil {
		return true
	}

	// Check Status.
	os := NewReflectedStatusAccessor(old)
	ns := NewReflectedStatusAccessor(new)
	oStatus := os.GetStatus()
	nStatus := ns.GetStatus()

	if !equality.Semantic.DeepEqual(oStatus, nStatus) {
		return true
	}

	// Check finalizers.
	of := NewReflectedFinalizersAccessor(old)
	nf := NewReflectedFinalizersAccessor(new)
	oFinalizers := of.GetFinalizers()
	nFinalizers := nf.GetFinalizers()

	if !equality.Semantic.DeepEqual(oFinalizers, nFinalizers) {
		return true
	}

	return false
}

func (r *Reconciler) update(ctx context.Context, request reconcile.Request, object runtime.Object) (runtime.Object, error) {
	freshObj := r.provider.Parent.DeepCopyObject()
	if err := r.client.Get(ctx, request.NamespacedName, freshObj); err != nil {
		return nil, err
	}

	// Status
	freshStatus := NewReflectedStatusAccessor(freshObj)
	orgStatus := NewReflectedStatusAccessor(object)
	freshStatus.SetStatus(orgStatus.GetStatus())

	// Finalizers
	freshFinalizers := NewReflectedFinalizersAccessor(freshObj)
	orgFinalizers := NewReflectedFinalizersAccessor(object)
	freshFinalizers.SetFinalizers(orgFinalizers.GetFinalizers())

	// Until #38113 is merged, we must use Update instead of UpdateStatus to
	// update the Status block of the Source resource. UpdateStatus will not
	// allow changes to the Spec of the resource, which is ideal for ensuring
	// nothing other than resource status has been updated.
	if err := r.client.Update(ctx, freshObj); err != nil {
		return nil, err
	}
	return freshObj, nil
}
