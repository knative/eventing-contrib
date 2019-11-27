/*
Copyright 2019 The Knative Authors

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

package rabbitmq

import (
	"context"
	"k8s.io/client-go/tools/cache"
	rabbitmqclient "knative.dev/eventing-contrib/rabbitmq/source/pkg/client/injection/client"
	rabbitmqinformer "knative.dev/eventing-contrib/rabbitmq/source/pkg/client/injection/informers/sources/v1alpha1/rabbitmqsource"
	"knative.dev/eventing-contrib/rabbitmq/source/pkg/client/injection/reconciler/sources/v1alpha1/rabbitmqsource"
	"knative.dev/eventing/pkg/apis/sources/v1alpha1"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/metrics"
	"knative.dev/pkg/resolver"
	"os"
)

const (
	controllerAgentName = "rabbitmq-source-controller"
)

func NewController(
	ctx context.Context,
	cmw configmap.Watcher) *controller.Impl {

	raImage, defined := os.LookupEnv(raImageEnvVar)
	if !defined {
		logging.FromContext(ctx).Errorf("required environment variable '%s' not defined", raImageEnvVar)
		return nil
	}

	rabbitmqInformer := rabbitmqinformer.Get(ctx)
	deploymentInformer := deploymentinformer.Get(ctx)

	c := &Reconciler{
		KubeClientSet: 		 kubeclient.Get(ctx),
		rabbitmqClientSet:   rabbitmqclient.Get(ctx),
		rabbitmqLister:      rabbitmqInformer.Lister(),
		deploymentLister:    deploymentInformer.Lister(),
		receiveAdapterImage: raImage,
		loggingContext:      ctx,
	}

	impl := rabbitmqsource.NewImpl(ctx, c)
	c.sinkResolver = resolver.NewURIResolver(ctx, impl.EnqueueKey)

	logging.FromContext(ctx).Info("Setting up rabbitmq event handlers")

	rabbitmqInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterGroupKind(v1alpha1.Kind("RabbitmqSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	cmw.Watch(logging.ConfigMapName(), c.UpdateFromLoggingConfigMap)
	cmw.Watch(metrics.ConfigMapName(), c.UpdateFromMetricsConfigMap)
	return impl
}