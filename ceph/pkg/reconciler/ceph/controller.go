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

package ceph

import (
	"context"

	reconcilersource "knative.dev/eventing/pkg/reconciler/source"

	"knative.dev/eventing-contrib/ceph/pkg/apis/sources/v1alpha1"

	"github.com/kelseyhightower/envconfig"
	"k8s.io/client-go/tools/cache"

	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"

	"knative.dev/eventing-contrib/ceph/pkg/reconciler"

	eventingclient "knative.dev/eventing/pkg/client/injection/client"
	sinkbindinginformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1beta1/broker"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	cephsourceinformer "knative.dev/eventing-contrib/ceph/pkg/client/injection/informers/sources/v1alpha1/cephsource"
	"knative.dev/eventing-contrib/ceph/pkg/client/injection/reconciler/sources/v1alpha1/cephsource"
)

// NewController initializes the controller and is called by the generated code
// Registers event handlers to enqueue events
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	deploymentInformer := deploymentinformer.Get(ctx)
	sinkBindingInformer := sinkbindinginformer.Get(ctx)
	cephSourceInformer := cephsourceinformer.Get(ctx)

	r := &Reconciler{
		dr:  &reconciler.DeploymentReconciler{KubeClientSet: kubeclient.Get(ctx)},
		sbr: &reconciler.SinkBindingReconciler{EventingClientSet: eventingclient.Get(ctx)},
		// Config accessor takes care of tracing/config/logging config propagation to the receive adapter
		configAccessor: reconcilersource.WatchConfigurations(ctx, "ceph-source", cmw),
	}
	if err := envconfig.Process("", r); err != nil {
		logging.FromContext(ctx).Panicf("required environment variable is not defined: %v", err)
	}

	impl := cephsource.NewImpl(ctx, r)

	logging.FromContext(ctx).Info("Setting up event handlers")

	cephSourceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(v1alpha1.Kind("CephSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	sinkBindingInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterControllerGK(v1alpha1.Kind("CephSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	return impl
}
