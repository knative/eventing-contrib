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

package binding

import (
	"context"

	kfkinformer "knative.dev/eventing-contrib/kafka/source/pkg/client/injection/informers/bindings/v1alpha1/kafkabinding"
	"knative.dev/pkg/client/injection/ducks/duck/v1/podspecable"
	"knative.dev/pkg/client/injection/kube/informers/core/v1/namespace"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"knative.dev/eventing-contrib/kafka/source/pkg/apis/bindings/v1alpha1"
	"knative.dev/pkg/apis/duck"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/tracker"
	"knative.dev/pkg/webhook/psbinding"
)

const (
	controllerAgentName = "kafkabinding-controller"
)

// NewController returns a new KafkaBinding reconciler.
func NewController(
	ctx context.Context,
	cmw configmap.Watcher,
) *controller.Impl {
	logger := logging.FromContext(ctx)

	kfkInformer := kfkinformer.Get(ctx)
	dc := dynamicclient.Get(ctx)
	psInformerFactory := podspecable.Get(ctx)
	namespaceInformer := namespace.Get(ctx)

	c := &psbinding.BaseReconciler{
		GVR: v1alpha1.SchemeGroupVersion.WithResource("kafkabindings"),
		Get: func(namespace string, name string) (psbinding.Bindable, error) {
			return kfkInformer.Lister().KafkaBindings(namespace).Get(name)
		},
		DynamicClient: dc,
		Recorder: record.NewBroadcaster().NewRecorder(
			scheme.Scheme, corev1.EventSource{Component: controllerAgentName}),
		NamespaceLister: namespaceInformer.Lister(),
	}
	impl := controller.NewImpl(c, logger, "KafkaBindings")

	logger.Info("Setting up event handlers")

	kfkInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))
	namespaceInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

	c.Tracker = tracker.New(impl.EnqueueKey, controller.GetTrackerLease(ctx))
	c.Factory = &duck.CachedInformerFactory{
		Delegate: &duck.EnqueueInformerFactory{
			Delegate:     psInformerFactory,
			EventHandler: controller.HandleAll(c.Tracker.OnChanged),
		},
	}

	return impl
}

func ListAll(ctx context.Context, handler cache.ResourceEventHandler) psbinding.ListAll {
	fbInformer := kfkinformer.Get(ctx)

	// Whenever a KafkaBinding changes our webhook programming might change.
	fbInformer.Informer().AddEventHandler(handler)

	return func() ([]psbinding.Bindable, error) {
		l, err := fbInformer.Lister().List(labels.Everything())
		if err != nil {
			return nil, err
		}
		bl := make([]psbinding.Bindable, 0, len(l))
		for _, elt := range l {
			bl = append(bl, elt)
		}
		return bl, nil
	}

}
