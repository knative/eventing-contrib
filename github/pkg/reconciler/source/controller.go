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
	serviceclient "knative.dev/serving/pkg/client/injection/client"
	kserviceinformer "knative.dev/serving/pkg/client/injection/informers/serving/v1/service"

	sourcescheme "knative.dev/eventing-contrib/github/pkg/client/clientset/versioned/scheme"
	githubinformer "knative.dev/eventing-contrib/github/pkg/client/injection/informers/sources/v1alpha1/githubsource"
	ghreconciler "knative.dev/eventing-contrib/github/pkg/client/injection/reconciler/sources/v1alpha1/githubsource"
)

// envConfig will be used to extract the required environment variables.
// If this configuration cannot be extracted, then NewController will panic.
type envConfig struct {
	Image string `envconfig:"GH_RA_IMAGE" required:"true"`

	// Add this for validation purpose only of validation.
	ControllerName string `envconfig:"CONTROLLER_NAME" required:"true"`
	ControllerUID  string `envconfig:"CONTROLLER_UID" required:"true"`
}

func NewController(
	ctx context.Context,
	_ configmap.Watcher,
) *controller.Impl {
	env := &envConfig{}
	if err := envconfig.Process("", env); err != nil {
		logging.FromContext(ctx).Panicf("unable to process GitHubSource's required environment variables: %v", err)
	}

	githubInformer := githubinformer.Get(ctx)
	serviceInformer := kserviceinformer.Get(ctx)

	r := &Reconciler{
		kubeClientSet:       kubeclient.Get(ctx),
		servingLister:       serviceInformer.Lister(),
		servingClientSet:    serviceclient.Get(ctx),
		webhookClient:       gitHubWebhookClient{},
		receiveAdapterImage: env.Image,
	}
	impl := ghreconciler.NewImpl(ctx, r)

	r.sinkResolver = resolver.NewURIResolver(ctx, impl.EnqueueKey)

	logging.FromContext(ctx).Info("Setting up GitHub event handlers")

	githubInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	// Tracker is used to notify us that the mt adapter KService has changed so that
	// we can reconcile all GitHubSources that depends on it
	r.tracker = tracker.New(impl.EnqueueKey, controller.GetTrackerLease(ctx))
	serviceInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.FilterWithNameAndNamespace(system.Namespace(), adapterName),
		Handler:    controller.HandleAll(r.tracker.OnChanged)})

	return impl
}

func init() {
	sourcescheme.AddToScheme(scheme.Scheme)
}
