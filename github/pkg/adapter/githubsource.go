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

package adapter

import (
	"context"
	"fmt"

	"k8s.io/client-go/kubernetes"
	"knative.dev/eventing-contrib/github/pkg/common"

	"go.uber.org/zap"
	clientset "knative.dev/eventing/pkg/client/clientset/versioned"
	"knative.dev/eventing/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"

	"knative.dev/eventing-contrib/github/pkg/apis/sources/v1alpha1"
	githubsourcereconciler "knative.dev/eventing-contrib/github/pkg/client/injection/reconciler/sources/v1alpha1/githubsource"
	sourceslisters "knative.dev/eventing-contrib/github/pkg/client/listers/sources/v1alpha1"
)

// Reconciler updates the internal Adapter cache GitHubSources
type Reconciler struct {
	kubeClientSet      kubernetes.Interface
	eventingClientSet  clientset.Interface
	githubsourceLister sourceslisters.GitHubSourceLister

	handler *Handler
}

// Check that our Reconciler implements ReconcileKind.
var _ githubsourcereconciler.Interface = (*Reconciler)(nil)

// Check that our Reconciler implements FinalizeKind.
var _ githubsourcereconciler.Finalizer = (*Reconciler)(nil)

func (r *Reconciler) ReconcileKind(ctx context.Context, source *v1alpha1.GitHubSource) pkgreconciler.Event {
	if !source.Status.IsReady() {
		return fmt.Errorf("GitHubSource is not ready. Cannot configure the adapter")
	}

	reconcileErr := r.reconcile(ctx, source)
	if reconcileErr != nil {
		logging.FromContext(ctx).Error("Error reconciling GitHubSource", zap.Error(reconcileErr))
	} else {
		logging.FromContext(ctx).Debug("GitHubSource reconciled")
	}
	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, source *v1alpha1.GitHubSource) error {
	logging.FromContext(ctx).Info("synchronizing githubsource")

	secretToken, err := common.SecretFrom(r.kubeClientSet, source.Namespace, source.Spec.SecretToken.SecretKeyRef)

	adapter, err := New(source.Status.SinkURI.String(), source.Spec.OwnerAndRepository, secretToken)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/%s/%s", source.Namespace, source.Name)
	r.handler.Register(path, adapter)
	return nil

}

func (r *Reconciler) FinalizeKind(ctx context.Context, source *v1alpha1.GitHubSource) pkgreconciler.Event {
	return nil
}
