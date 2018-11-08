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
	"github.com/knative/pkg/logging"
	"github.com/knative/pkg/signals"
	"go.uber.org/zap"
	"io/ioutil"
	"net/http"

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

	ctx context.Context
}

// Run starts the kubernetes receive adapter and sends events produced in the
// configured namespace and send them to the sink uri provided.
func (a *Adapter) Run(ctx context.Context) {
	a.ctx = ctx
	logger := logging.FromContext(ctx)

	// set up signals so we handle the first shutdown signal gracefully
	stopCh := signals.SetupSignalHandler()

	cfg, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		logger.Fatalf("error building kubeconfig: %v", zap.Error(err))
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logger.Fatalf("error building kubernetes clientset: %v", zap.Error(err))
	}

	eventsInformer := coreinformers.NewFilteredEventInformer(
		kubeClient, a.Namespace, 0, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, nil)

	eventsInformer.AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc:    a.addEvent,
		UpdateFunc: a.updateEvent,
	})

	logger.Debug("Starting eventsInformer...")
	go eventsInformer.Run(stopCh)

	logger.Debug("waiting for caches to sync...")
	if ok := cache.WaitForCacheSync(stopCh, eventsInformer.HasSynced); !ok {
		logger.Fatalf("Failed to wait for events cache to sync")
	}
	logger.Debug("caches synced...")
	<-stopCh
}

func (a *Adapter) updateEvent(old, new interface{}) {
	a.addEvent(new)
}

func (a *Adapter) addEvent(new interface{}) {
	logger := logging.FromContext(a.ctx)
	event := new.(*corev1.Event)
	logger.Debugf("GOT EVENT: %+v", event)
	a.postMessage(event)
}

func (a *Adapter) postMessage(m *corev1.Event) error {
	logger := logging.FromContext(a.ctx)

	ectx := cloudEventsContext(m)

	logger.Debugf("posting to %q", a.SinkURI)
	// Explicitly using Binary encoding so that Istio, et. al. can better inspect
	// event metadata.
	req, err := cloudevents.Binary.NewRequest(a.SinkURI, m, *ectx)
	if err != nil {
		logger.Errorf("failed to create http request: %v", zap.Error(err))
		return err
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		logger.Errorf("failed to do POST: %v", zap.Error(err))
		return err
	}
	defer resp.Body.Close()
	logger.Debugf("response Status: %s", resp.Status)
	body, _ := ioutil.ReadAll(resp.Body)
	logger.Debugf("response Body: %s", string(body))
	return nil
}
