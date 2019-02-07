/*
Copyright 2018 The Knative Authors

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

package kubernetesevents

import (
	"context"
	"fmt"

	"github.com/knative/pkg/logging"
	"go.uber.org/zap"

	"github.com/knative/pkg/cloudevents"
	corev1 "k8s.io/api/core/v1"
	coreinformers "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
)

const (
	eventType = "dev.knative.k8s.event"
)

type Adapter struct {
	// Namespace is the kubernetes namespace to watch for events from.
	Namespace string
	// SinkURI is the URI messages will be forwarded on to.
	SinkURI string

	// client sends cloudevents.
	client *cloudevents.Client

	kubeClient kubernetes.Interface
}

func (a *Adapter) makeKubeClient() error {
	cfg, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		return err
	}
	a.kubeClient, err = kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
	}
	return nil
}

// Run starts the kubernetes receive adapter and sends events produced in the
// configured namespace and send them to the sink uri provided.
func (a *Adapter) Start(stopCh <-chan struct{}) error {
	logger := logging.FromContext(context.TODO())

	if a.kubeClient == nil {
		if err := a.makeKubeClient(); err != nil {
			return err
		}
	}

	eventsInformer := coreinformers.NewFilteredEventInformer(
		a.kubeClient, a.Namespace, 0, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, nil)

	eventsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    a.addEvent,
		UpdateFunc: a.updateEvent,
	})

	logger.Debug("Starting eventsInformer...")
	stop := make(chan struct{})
	go eventsInformer.Run(stop)

	logger.Debug("waiting for caches to sync...")
	if ok := cache.WaitForCacheSync(stopCh, eventsInformer.HasSynced); !ok {
		return fmt.Errorf("failed to wait for events cache to sync")
	}
	logger.Debug("caches synced...")
	<-stopCh
	stop <- struct{}{}
	return nil
}

func (a *Adapter) updateEvent(old, new interface{}) {
	a.addEvent(new)
}

func (a *Adapter) addEvent(new interface{}) {
	event := new.(*corev1.Event)
	logger := logging.FromContext(context.TODO()).With(zap.Any("eventID", event.ObjectMeta.UID), zap.Any("sink", a.SinkURI))
	logger.Debug("GOT EVENT", event)

	if a.client == nil {
		a.client = cloudevents.NewClient(a.SinkURI, cloudevents.Builder{EventType: eventType})
	}

	if err := a.client.Send(event, cloudEventOverrides(event)); err != nil {
		logger.Infof("Event delivery failed: %s", err)
	}
}
