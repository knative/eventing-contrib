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

package source

import (
	"context"

	"go.uber.org/zap"

	"github.com/kelseyhightower/envconfig"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"

	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/resolver"
	"knative.dev/pkg/system"
	"knative.dev/pkg/tracker"

	network "knative.dev/networking/pkg"

	networkingclient "knative.dev/networking/pkg/client/injection/client"
	networkinginformer "knative.dev/networking/pkg/client/injection/informers/networking/v1alpha1/ingress"
	serviceclient "knative.dev/serving/pkg/client/injection/client"
	kserviceinformer "knative.dev/serving/pkg/client/injection/informers/serving/v1/service"
	"knative.dev/serving/pkg/reconciler/route/config"

	"knative.dev/eventing-contrib/github/pkg/apis/sources/v1alpha1"
	sourcescheme "knative.dev/eventing-contrib/github/pkg/client/clientset/versioned/scheme"
	githubinformer "knative.dev/eventing-contrib/github/pkg/client/injection/informers/sources/v1alpha1/githubsource"
	ghreconciler "knative.dev/eventing-contrib/github/pkg/client/injection/reconciler/sources/v1alpha1/githubsource"
)

// envConfig will be used to extract the required environment variables.
// If this configuration cannot be extracted, then NewController will panic.
type envConfig struct {
	Image            string `envconfig:"GH_RA_IMAGE"`
	EnableVanityURL  bool   `envconfig:"ENABLE_VANITY_URL"`
	ServingNamespace string `envconfig:"SERVING_NAMESPACE"`
}

func NewController(ctx context.Context, _ configmap.Watcher) *controller.Impl {
	logger := logging.FromContext(ctx)
	env := &envConfig{}
	if err := envconfig.Process("", env); err != nil {
		logger.Panicf("unable to process GitHubSource's required environment variables: %v", err)
	}

	if env.EnableVanityURL {
		if env.ServingNamespace == "" {
			logger.Panicf("unable to process GitHubSource's required environment variables: SERVING_NAMESPACE is missing")
		}
	}

	githubInformer := githubinformer.Get(ctx)
	serviceInformer := kserviceinformer.Get(ctx)

	r := &Reconciler{
		kubeClientSet:       kubeclient.Get(ctx),
		servingLister:       serviceInformer.Lister(),
		servingClientSet:    serviceclient.Get(ctx),
		webhookClient:       gitHubWebhookClient{},
		receiveAdapterImage: env.Image, // can be empty
		enableVanityURL:     env.EnableVanityURL,
	}

	impl := ghreconciler.NewImpl(ctx, r, func(impl *controller.Impl) controller.Options {
		if env.EnableVanityURL {
			// Configure networking informer and clienset
			r.networkingLister = networkinginformer.Get(ctx).Lister()
			r.networkingClientSet = networkingclient.Get(ctx)

			// Watch for Knative Serving network configurations. First need a new ConfigMapWatcher
			kc := kubeclient.Get(ctx)
			scmw := configmap.NewInformedWatcher(kc, env.ServingNamespace)

			logger.Info("Setting up networking ConfigMap receivers")
			configsToResync := []interface{}{
				&network.Config{},
				&config.Domain{},
			}
			resync := configmap.TypeFilter(configsToResync...)(func(string, interface{}) {
				impl.GlobalResync(githubInformer.Informer())
			})
			configStore := config.NewStore(ctx, resync)
			configStore.WatchConfigs(scmw)

			logger.Info("Starting serving configuration manager...")
			if err := scmw.Start(ctx.Done()); err != nil {
				logger.Fatalw("Failed to start serving configuration manager", zap.Error(err))
			}

			return controller.Options{ConfigStore: configStore}
		}
		return controller.Options{}
	})

	r.sinkResolver = resolver.NewURIResolver(ctx, impl.EnqueueKey)

	logger.Info("Setting up GitHub event handlers")

	// Watch for changes from any GitHubSource object
	githubInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	if r.receiveAdapterImage == "" {
		// Running in multi-tenant mode.

		// Tracker is used to notify us that the mt receive adapter has changed so that
		// we can reconcile all GitHubSources that depends on it
		r.tracker = tracker.New(impl.EnqueueKey, controller.GetTrackerLease(ctx))

		// Watch for changes from the multi-tenant receive adapter
		serviceInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterWithNameAndNamespace(system.Namespace(), adapterName),
			Handler:    controller.HandleAll(r.tracker.OnChanged)})
	} else {
		// Watch for changes from any Knative service owned by a GitHubSource
		serviceInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
			FilterFunc: controller.FilterControllerGK(v1alpha1.Kind("GitHubSource")),
			Handler:    controller.HandleAll(impl.EnqueueControllerOf),
		})
	}

	return impl
}

func init() {
	sourcescheme.AddToScheme(scheme.Scheme)
}
