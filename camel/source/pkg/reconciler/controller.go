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
	context "context"
	"knative.dev/pkg/injection/clients/dynamicclient"

	camelsource "knative.dev/eventing-contrib/camel/source/pkg/client/injection/informers/sources/v1alpha1/camelsource"
	v1alpha1camelsource "knative.dev/eventing-contrib/camel/source/pkg/client/injection/reconciler/sources/v1alpha1/camelsource"
	configmap "knative.dev/pkg/configmap"
	controller "knative.dev/pkg/controller"
	logging "knative.dev/pkg/logging"
	"knative.dev/pkg/resolver"
)

// NewController creates a Reconciler for CamelSource and returns the result of NewImpl.
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	logger := logging.FromContext(ctx)

	camelsourceInformer := camelsource.Get(ctx)

	// TODO: setup additional informers here.

	r := &Reconciler{
		dynamicClientSet: dynamicclient.Get(ctx),
	}
	impl := v1alpha1camelsource.NewImpl(ctx, r)

	// Set sink resolver.
	r.sinkResolver = resolver.NewURIResolver(ctx, impl.EnqueueKey)

	logger.Info("Setting up event handlers.")

	camelsourceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	// TODO: add additional informer event handlers here.

	return impl
}
