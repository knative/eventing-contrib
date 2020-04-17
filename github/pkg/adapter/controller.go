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
	"net/http"

	"go.uber.org/zap"

	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	"knative.dev/eventing/pkg/tracing"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	tracingconfig "knative.dev/pkg/tracing/config"

	githubsourceinformer "knative.dev/eventing-contrib/github/pkg/client/injection/informers/sources/v1alpha1/githubsource"
	githubsourcereconciler "knative.dev/eventing-contrib/github/pkg/client/injection/reconciler/sources/v1alpha1/githubsource"
)

// NewController initializes the controller and is called by the generated code.
// Registers event handlers to enqueue events.
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	logger := logging.FromContext(ctx)

	// Setup trace publishing.
	iw := cmw.(*configmap.InformedWatcher)
	if err := tracing.SetupDynamicPublishing(logger, iw, "github-source-adapter", tracingconfig.ConfigName); err != nil {
		logger.Fatalw("Error setting up trace publishing", zap.Error(err))
	}

	githubsourceInformer := githubsourceinformer.Get(ctx)

	r := &Reconciler{
		kubeClientSet:      kubeclient.Get(ctx),
		eventingClientSet:  eventingclient.Get(ctx),
		githubsourceLister: githubsourceInformer.Lister(),
		handler:            NewHandler(logger),
	}

	impl := githubsourcereconciler.NewImpl(ctx, r)

	logger.Info("Setting up event handlers")

	// Watch for githubsource objects
	githubsourceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	// Start our GitHub webhook handler
	go func() {
		http.ListenAndServe(":8080", r.handler)
	}()

	return impl
}
