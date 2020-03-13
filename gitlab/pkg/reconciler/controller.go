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
	"os"

	//k8s.io imports
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"

	//Injection imports
	"knative.dev/eventing-contrib/gitlab/pkg/apis/sources/v1alpha1"
	sourcescheme "knative.dev/eventing-contrib/gitlab/pkg/client/clientset/versioned/scheme"
	gitlabclient "knative.dev/eventing-contrib/gitlab/pkg/client/injection/client"
	gitlabinformer "knative.dev/eventing-contrib/gitlab/pkg/client/injection/informers/sources/v1alpha1/gitlabsource"
	eventtypeinformer "knative.dev/eventing/pkg/client/injection/informers/eventing/v1alpha1/eventtype"
	serviceclient "knative.dev/serving/pkg/client/injection/client"
	kserviceinformer "knative.dev/serving/pkg/client/injection/informers/serving/v1/service"

	//knative.dev/eventing imports
	"knative.dev/eventing/pkg/reconciler"

	//knative.dev/pkg imports
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/resolver"
)

// NewController returns the controller implementation with reconciler structure and logger
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {

	raImage := os.Getenv(raImageEnvVar)
	if raImage == "" {
		logging.FromContext(ctx).Errorf("required environment variable %q not set", raImageEnvVar)
		return nil
	}

	gitlabInformer := gitlabinformer.Get(ctx)
	eventTypeInformer := eventtypeinformer.Get(ctx)
	serviceInformer := kserviceinformer.Get(ctx)

	r := &Reconciler{
		Base:                reconciler.NewBase(ctx, controllerAgentName, cmw),
		servingLister:       serviceInformer.Lister(),
		servingClientSet:    serviceclient.Get(ctx),
		gitlabClientSet:     gitlabclient.Get(ctx),
		gitlabLister:        gitlabInformer.Lister(),
		receiveAdapterImage: raImage,
		eventTypeLister:     eventTypeInformer.Lister(),
		loggingContext:      ctx,
	}

	impl := controller.NewImpl(r, r.Logger, "GitLabSource")
	r.sinkResolver = resolver.NewURIResolver(ctx, impl.EnqueueKey)

	r.Logger.Info("Setting up GitLab event handlers")

	gitlabInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	serviceInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("GitLabSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	eventTypeInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: controller.Filter(v1alpha1.SchemeGroupVersion.WithKind("GitLabSource")),
		Handler:    controller.HandleAll(impl.EnqueueControllerOf),
	})

	cmw.Watch(logging.ConfigMapName(), r.UpdateFromLoggingConfigMap)
	return impl

}

func init() {
	sourcescheme.AddToScheme(scheme.Scheme)
}
