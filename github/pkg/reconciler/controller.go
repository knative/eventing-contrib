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

package github

import (
	"context"
	"os"

	//k8s.io imports
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"

	//Injection imports
	sourcescheme "knative.dev/eventing-contrib/github/pkg/client/clientset/versioned/scheme"
	githubclient "knative.dev/eventing-contrib/github/pkg/client/injection/client"
	githubinformer "knative.dev/eventing-contrib/github/pkg/client/injection/informers/sources/v1alpha1/githubsource"
	eventtypeinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/eventtype"
	deploymentinformer "knative.dev/pkg/client/injection/kube/informers/apps/v1/deployment"
	serviceclient "knative.dev/serving/pkg/client/injection/client"
	kserviceinformer "knative.dev/serving/pkg/client/injection/informers/serving/v1alpha1/service"

	//knative.dev/eventing imports
	"knative.dev/eventing-contrib/pkg/apis/sources/v1alpha1"
	"knative.dev/eventing/pkg/reconciler"

	//knative.dev/pkg imports
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/resolver"
)

func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	raImage, defined := os.LookupEnv(raImageEnvVar)
	if !defined {
		logging.FromContext(ctx).Errorf("required environment variable '%s' not defined", raImageEnvVar)
		return nil
	}

	githubInformer := githubinformer.Get(ctx)
	eventTypeInformer := eventtypeinformer.Get(ctx)
	deploymentInformer := deploymentinformer.Get(ctx)
	serviceInformer := kserviceinformer.Get(ctx)

	r := &Reconciler{
		Base:                reconciler.NewBase(ctx, controllerAgentName, cmw),
		servingLister:       serviceInformer.Lister(),
		servingClientSet:    serviceclient.Get(ctx),
		githubClientSet:     githubclient.Get(ctx),
		githubLister:        githubInformer.Lister(),
		deploymentLister:    deploymentInformer.Lister(),
		webhookClient:       gitHubWebhookClient{},
		receiveAdapterImage: raImage,
		eventTypeLister:     eventTypeInformer.Lister(),
		loggingContext:      ctx,
	}

	impl := controller.NewImpl(r, r.Logger, "GitHubSource")
	r.sinkResolver = resolver.NewURIResolver(ctx, impl.EnqueueKey)

	r.Logger.Info("Setting up GitHub event handlers")

	githubInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	serviceInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("GitHubSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	deploymentInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("GitHubSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	eventTypeInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("GitHubSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	cmw.Watch(logging.ConfigMapName(), r.UpdateFromLoggingConfigMap)
	return impl

}

func init() {
	sourcescheme.AddToScheme(scheme.Scheme)
}
