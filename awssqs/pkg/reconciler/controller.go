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
	"os"

	"k8s.io/client-go/tools/cache"
	"knative.dev/eventing-contrib/awssqs/pkg/apis/sources/v1alpha1"
	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	"knative.dev/pkg/resolver"

	awssqssource "knative.dev/eventing-contrib/awssqs/pkg/client/injection/informers/sources/v1alpha1/awssqssource"
	v1alpha1awssqssource "knative.dev/eventing-contrib/awssqs/pkg/client/injection/reconciler/sources/v1alpha1/awssqssource"
	configmap "knative.dev/pkg/configmap"
	controller "knative.dev/pkg/controller"
	logging "knative.dev/pkg/logging"
)

const (
	raImageEnvVar = "AWSSQS_RA_IMAGE"
)

// NewController creates a Reconciler for AwsSqsSource and returns the result of NewImpl.
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	logger := logging.FromContext(ctx)

	awssqssourceInformer := awssqssource.Get(ctx)
	deploymentInformer := deploymentinformer.Get(ctx)

	raImage, defined := os.LookupEnv(raImageEnvVar)
	if !defined {
		logger.Fatalf("required environment variable '%s' not defined", raImageEnvVar)
	}

	r := &Reconciler{
		kubeClientSet:       kubeclient.Get(ctx),
		eventingClientSet:   eventingclient.Get(ctx),
		receiveAdapterImage: raImage,
	}
	impl := v1alpha1awssqssource.NewImpl(ctx, r)

	// Set sink resolver.
	r.sinkResolver = resolver.NewURIResolver(ctx, impl.EnqueueKey)

	logger.Info("Setting up event handlers.")

	awssqssourceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterGroupVersionKind(v1alpha1.SchemeGroupVersion.WithKind("AwsSqsSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	return impl
}
