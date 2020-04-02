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

package controller

import (
	"context"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	"knative.dev/pkg/client/injection/kube/informers/core/v1/endpoints"
	"knative.dev/pkg/client/injection/kube/informers/core/v1/service"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"

	"k8s.io/client-go/tools/cache"

	"knative.dev/eventing-contrib/natss/pkg/client/injection/informers/messaging/v1alpha1/natsschannel"
	natssChannelReconciler "knative.dev/eventing-contrib/natss/pkg/client/injection/reconciler/messaging/v1alpha1/natsschannel"
)

// NewController initializes the controller and is called by the generated code.
// Registers event handlers to enqueue events.
func NewController(ctx context.Context) *controller.Impl {

	logger := logging.FromContext(ctx)
	channelInformer := natsschannel.Get(ctx)
	deploymentInformer := deploymentinformer.Get(ctx)
	serviceInformer := service.Get(ctx)
	endpointsInformer := endpoints.Get(ctx)
	kubeClient := kubeclient.Get(ctx)

	r := &Reconciler{
		kubeClientSet:            kubeClient,
		dispatcherNamespace:      system.Namespace(),
		dispatcherDeploymentName: dispatcherName,
		dispatcherServiceName:    dispatcherName,
		natsschannelLister:       channelInformer.Lister(),
		natsschannelInformer:     channelInformer.Informer(),
		deploymentLister:         deploymentInformer.Lister(),
		serviceLister:            serviceInformer.Lister(),
		endpointsLister:          endpointsInformer.Lister(),
	}

	impl := natssChannelReconciler.NewImpl(ctx, r)

	logger.Info("Setting up event handlers")
	channelInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	grCh := func(obj interface{}) {
		impl.GlobalResync(channelInformer.Informer())
	}
	filterFunc := controller.FilterWithNameAndNamespace(r.dispatcherNamespace, r.dispatcherDeploymentName)

	// Set up watches for dispatcher resources we care about, since any changes to these
	// resources will affect our Channels. So, set up a watch here, that will cause
	// a global Resync for all the channels to take stock of their health when these change.
	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: filterFunc,
		Handler:    controller.HandleAll(grCh),
	})
	serviceInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: filterFunc,
		Handler:    controller.HandleAll(grCh),
	})
	endpointsInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: filterFunc,
		Handler:    controller.HandleAll(grCh),
	})

	return impl
}
