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

	githubsourceinformer "knative.dev/eventing-contrib/github/pkg/client/injection/informers/sources/v1alpha1/githubsource"
	githubsourcereconciler "knative.dev/eventing-contrib/github/pkg/client/injection/reconciler/sources/v1alpha1/githubsource"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
)

// NewController initializes the controller and
// registers event handlers to enqueue events.
func NewController(ctx context.Context, router *Router) *controller.Impl {
	logger := logging.FromContext(ctx)

	r := &Reconciler{
		kubeClientSet: kubeclient.Get(ctx),
		router:        router,
	}

	impl := githubsourcereconciler.NewImpl(ctx, r)

	logger.Info("Setting up event handlers")

	// Watch for githubsource objects
	githubsourceInformer := githubsourceinformer.Get(ctx)
	githubsourceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	return impl
}
