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
	"io/ioutil"
	"net/http"

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
}

// Run starts the kubernetes receive adapter and sends events produced in the
// configured namespace and send them to the sink uri provided.
func (a *Adapter) Start(stopCh <-chan struct{}) error {
	logger := logging.FromContext(context.TODO())

	// set up signals so we handle the first shutdown signal gracefully

	cfg, err := clientcmd.BuildConfigFromFlags("", "")
	if err != nil {
		return err
	}

	kubeClient, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		return err
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
		return fmt.Errorf("failed to wait for events cache to sync")
	}
	logger.Debug("caches synced...")
	<-stopCh
	return nil
}

func (a *Adapter) updateEvent(old, new interface{}) {
	a.addEvent(new)
}

func (a *Adapter) addEvent(new interface{}) {
	event := new.(*corev1.Event)
	logger := logging.FromContext(context.TODO()).With(zap.Any("eventID", event.ObjectMeta.UID), zap.Any("sink", a.SinkURI))
	logger.Debug("GOT EVENT", event)
	if err := a.postMessage(logger, event); err != nil {
		logger.Info("Event delivery failed: %s", err)
	}
}

func (a *Adapter) postMessage(logger *zap.SugaredLogger, m *corev1.Event) error {
	ectx := cloudEventsContext(m)

	logger.Debug("posting to sinkURI")
	// Explicitly using Binary encoding so that Istio, et. al. can better inspect
	// event metadata.
	req, err := cloudevents.Binary.NewRequest(a.SinkURI, m, *ectx)
	if err != nil {
		return fmt.Errorf("Encoding failed: %s", err)
	}

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("POST failed: %s", err)
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)
	logger.Debugw("response", zap.Any("status", resp.Status), zap.Any("body", string(body)))

	// If the response is not within the 2xx range, return an error.
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return fmt.Errorf("[%d] unexpected response %q", resp.StatusCode, body)
	}

	return nil
}
