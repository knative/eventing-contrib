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

package mtadapter

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"k8s.io/client-go/kubernetes"

	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"

	"knative.dev/eventing-contrib/github/pkg/apis/sources/v1alpha1"
	githubsourcereconciler "knative.dev/eventing-contrib/github/pkg/client/injection/reconciler/sources/v1alpha1/githubsource"
	"knative.dev/eventing-contrib/github/pkg/common"
)

// Reconciler updates the internal Adapter cache GitHubSources
type Reconciler struct {
	kubeClientSet kubernetes.Interface
	router        *Router
}

// Check that our Reconciler implements ReconcileKind.
var _ githubsourcereconciler.Interface = (*Reconciler)(nil)

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
	secretToken, err := common.SecretFrom(r.kubeClientSet, source.Namespace, source.Spec.SecretToken.SecretKeyRef)
	if err != nil {
		return err
	}

	src := v1alpha1.GitHubEventSource(source.Spec.OwnerAndRepository)
	adapter := common.NewHandler(r.router.ceClient, source.Status.SinkURI.String(), src, secretToken, logging.FromContext(ctx))

	path := fmt.Sprintf("/%s/%s", source.Namespace, source.Name)
	r.router.Register(source.Name, source.Namespace, path, adapter)

	return nil
}
