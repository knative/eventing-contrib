/*
Copyright 2020 The Knative Authors.

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

package gitlab

import (
	"context"

	"github.com/kelseyhightower/envconfig"

	//k8s.io imports
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"

	//Injection imports
	"knative.dev/eventing-contrib/gitlab/pkg/apis/sources/v1alpha1"
	sourcescheme "knative.dev/eventing-contrib/gitlab/pkg/client/clientset/versioned/scheme"
	gitlabclient "knative.dev/eventing-contrib/gitlab/pkg/client/injection/client"
	gitlabinformer "knative.dev/eventing-contrib/gitlab/pkg/client/injection/informers/sources/v1alpha1/gitlabsource"
	v1alpha1gitlabsource "knative.dev/eventing-contrib/gitlab/pkg/client/injection/reconciler/sources/v1alpha1/gitlabsource"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	serviceclient "knative.dev/serving/pkg/client/injection/client"
	kserviceinformer "knative.dev/serving/pkg/client/injection/informers/serving/v1/service"

	//knative.dev/pkg imports
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/resolver"
)

type envConfig struct {
	Image string `envconfig:"GL_RA_IMAGE" required:"true"`
}

// NewController returns the controller implementation with reconciler structure and logger
func NewController(
	ctx context.Context,
	_ configmap.Watcher,
) *controller.Impl {
	gitlabInformer := gitlabinformer.Get(ctx)
	serviceInformer := kserviceinformer.Get(ctx)

	r := &Reconciler{
		kubeClientSet:    kubeclient.Get(ctx),
		servingLister:    serviceInformer.Lister(),
		servingClientSet: serviceclient.Get(ctx),
		gitlabClientSet:  gitlabclient.Get(ctx),
		gitlabLister:     gitlabInformer.Lister(),
		loggingContext:   ctx,
	}

	env := &envConfig{}
	if err := envconfig.Process("", env); err != nil {
		logging.FromContext(ctx).Panicf("unable to process GitLabSource's required environment variables: %v", err)
	}
	r.receiveAdapterImage = env.Image

	impl := v1alpha1gitlabsource.NewImpl(ctx, r)
	r.sinkResolver = resolver.NewURIResolver(ctx, impl.EnqueueKey)

	logging.FromContext(ctx).Info("Setting up GitLab event handlers")

	gitlabInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	serviceInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("GitLabSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	return impl

}

func init() {
	sourcescheme.AddToScheme(scheme.Scheme)
}
