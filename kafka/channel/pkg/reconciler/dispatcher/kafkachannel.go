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

package controller

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"

	eventingduckv1 "knative.dev/eventing/pkg/apis/duck/v1"
	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/pkg/channel/fanout"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/tracing"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/logging"
	pkgreconciler "knative.dev/pkg/reconciler"

	"knative.dev/eventing-contrib/kafka/channel/pkg/apis/messaging/v1beta1"
	kafkaclientset "knative.dev/eventing-contrib/kafka/channel/pkg/client/clientset/versioned"
	kafkaScheme "knative.dev/eventing-contrib/kafka/channel/pkg/client/clientset/versioned/scheme"
	kafkaclientsetinjection "knative.dev/eventing-contrib/kafka/channel/pkg/client/injection/client"
	"knative.dev/eventing-contrib/kafka/channel/pkg/client/injection/informers/messaging/v1beta1/kafkachannel"
	kafkachannelreconciler "knative.dev/eventing-contrib/kafka/channel/pkg/client/injection/reconciler/messaging/v1beta1/kafkachannel"
	listers "knative.dev/eventing-contrib/kafka/channel/pkg/client/listers/messaging/v1beta1"
	"knative.dev/eventing-contrib/kafka/channel/pkg/dispatcher"
	"knative.dev/eventing-contrib/kafka/channel/pkg/utils"
)

func init() {
	// Add run types to the default Kubernetes Scheme so Events can be
	// logged for run types.
	_ = kafkaScheme.AddToScheme(scheme.Scheme)
}

// Reconciler reconciles Kafka Channels.
type Reconciler struct {
	kafkaDispatcher *dispatcher.KafkaDispatcher

	kafkaClientSet       kafkaclientset.Interface
	kafkachannelLister   listers.KafkaChannelLister
	kafkachannelInformer cache.SharedIndexInformer
	impl                 *controller.Impl
}

// Check that our Reconciler implements controller.Reconciler.
var _ kafkachannelreconciler.Interface = (*Reconciler)(nil)

// NewController initializes the controller and is called by the generated code.
// Registers event handlers to enqueue events.
func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	logger := logging.FromContext(ctx)

	err := tracing.SetupDynamicPublishing(logger, cmw.(*configmap.InformedWatcher), "kafka-ch-dispatcher", "config-tracing")
	if err != nil {
		logger.Fatalw("unable to setup tracing", zap.Error(err))
	}

	configMap, err := configmap.Load("/etc/config-kafka")
	if err != nil {
		logger.Fatalw("error loading configuration", zap.Error(err))
	}

	kafkaConfig, err := utils.GetKafkaConfig(configMap)
	if err != nil {
		logger.Fatalw("Error loading kafka config", zap.Error(err))
	}

	connectionArgs := &kncloudevents.ConnectionArgs{
		MaxIdleConns:        int(kafkaConfig.MaxIdleConns),
		MaxIdleConnsPerHost: int(kafkaConfig.MaxIdleConnsPerHost),
	}

	kafkaChannelInformer := kafkachannel.Get(ctx)
	args := &dispatcher.KafkaDispatcherArgs{
		KnCEConnectionArgs: connectionArgs,
		ClientID:           "kafka-ch-dispatcher",
		Brokers:            kafkaConfig.Brokers,
		TopicFunc:          utils.TopicName,
		Logger:             logger,
	}
	kafkaDispatcher, err := dispatcher.NewDispatcher(ctx, args)
	if err != nil {
		logger.Fatalw("Unable to create kafka dispatcher", zap.Error(err))
	}
	logger.Info("Starting the Kafka dispatcher")
	logger.Infow("Kafka broker configuration", zap.Strings(utils.BrokerConfigMapKey, kafkaConfig.Brokers))

	r := &Reconciler{
		kafkaDispatcher:      kafkaDispatcher,
		kafkaClientSet:       kafkaclientsetinjection.Get(ctx),
		kafkachannelLister:   kafkaChannelInformer.Lister(),
		kafkachannelInformer: kafkaChannelInformer.Informer(),
	}
	r.impl = kafkachannelreconciler.NewImpl(ctx, r)

	logger.Info("Setting up event handlers")

	// Watch for kafka channels.
	kafkaChannelInformer.Informer().AddEventHandler(
		cache.FilteringResourceEventHandler{
			FilterFunc: filterWithAnnotation(injection.HasNamespaceScope(ctx)),
			Handler:    controller.HandleAll(r.impl.Enqueue),
		})

	logger.Info("Starting dispatcher.")
	go func() {
		if err := kafkaDispatcher.Start(ctx); err != nil {
			logger.Errorw("Cannot start dispatcher", zap.Error(err))
		}
	}()

	return r.impl
}

func filterWithAnnotation(namespaced bool) func(obj interface{}) bool {
	if namespaced {
		return pkgreconciler.AnnotationFilterFunc(eventing.ScopeAnnotationKey, "namespace", false)
	}
	return pkgreconciler.AnnotationFilterFunc(eventing.ScopeAnnotationKey, "cluster", true)
}

func (r *Reconciler) ReconcileKind(ctx context.Context, kc *v1beta1.KafkaChannel) pkgreconciler.Event {
	channels, err := r.kafkachannelLister.List(labels.Everything())
	if err != nil {
		logging.FromContext(ctx).Error("Error listing kafka channels")
		return err
	}

	// TODO: revisit this code. Instead of reading all channels and updating consumers and hostToChannel map for all
	// why not just reconcile the current channel. With this the UpdateKafkaConsumers can now return SubscribableStatus
	// for the subscriptions on the channel that is being reconciled.
	kafkaChannels := make([]*v1beta1.KafkaChannel, 0)
	for _, channel := range channels {
		if channel.Status.IsReady() {
			kafkaChannels = append(kafkaChannels, channel)
		}
	}
	config := r.newConfigFromKafkaChannels(kafkaChannels)
	if err := r.kafkaDispatcher.UpdateHostToChannelMap(config); err != nil {
		logging.FromContext(ctx).Error("Error updating host to channel map in dispatcher")
		return err
	}

	failedSubscriptions, err := r.kafkaDispatcher.UpdateKafkaConsumers(config)
	if err != nil {
		logging.FromContext(ctx).Error("Error updating kafka consumers in dispatcher")
		return err
	}
	kc.Status.SubscribableStatus = r.createSubscribableStatus(&kc.Spec.SubscribableSpec, failedSubscriptions)
	if len(failedSubscriptions) > 0 {
		logging.FromContext(ctx).Error("Some kafka subscriptions failed to subscribe")
		return fmt.Errorf("Some kafka subscriptions failed to subscribe")
	}
	return nil
}

func (r *Reconciler) createSubscribableStatus(subscribable *eventingduckv1.SubscribableSpec, failedSubscriptions map[types.UID]error) eventingduckv1.SubscribableStatus {
	if subscribable == nil {
		return eventingduckv1.SubscribableStatus{}
	}
	subscriberStatus := make([]eventingduckv1.SubscriberStatus, 0)
	for _, sub := range subscribable.Subscribers {
		status := eventingduckv1.SubscriberStatus{
			UID:                sub.UID,
			ObservedGeneration: sub.Generation,
			Ready:              corev1.ConditionTrue,
		}
		if err, ok := failedSubscriptions[sub.UID]; ok {
			status.Ready = corev1.ConditionFalse
			status.Message = err.Error()
		}
		subscriberStatus = append(subscriberStatus, status)
	}
	return eventingduckv1.SubscribableStatus{
		Subscribers: subscriberStatus,
	}
}

// newConfigFromKafkaChannels creates a new Config from the list of kafka channels.
func (r *Reconciler) newChannelConfigFromKafkaChannel(c *v1beta1.KafkaChannel) *dispatcher.ChannelConfig {
	channelConfig := dispatcher.ChannelConfig{
		Namespace: c.Namespace,
		Name:      c.Name,
		HostName:  c.Status.Address.URL.Host,
	}
	if c.Spec.SubscribableSpec.Subscribers != nil {
		newSubs := make([]dispatcher.Subscription, 0, len(c.Spec.SubscribableSpec.Subscribers))
		for _, source := range c.Spec.SubscribableSpec.Subscribers {
			innerSub, _ := fanout.SubscriberSpecToFanoutConfig(source)

			newSubs = append(newSubs, dispatcher.Subscription{
				Subscription: *innerSub,
				UID:          source.UID,
			})
		}
		channelConfig.Subscriptions = newSubs
	}

	return &channelConfig
}

// newConfigFromKafkaChannels creates a new Config from the list of kafka channels.
func (r *Reconciler) newConfigFromKafkaChannels(channels []*v1beta1.KafkaChannel) *dispatcher.Config {
	cc := make([]dispatcher.ChannelConfig, 0)
	for _, c := range channels {
		channelConfig := r.newChannelConfigFromKafkaChannel(c)
		cc = append(cc, *channelConfig)
	}
	return &dispatcher.Config{
		ChannelConfigs: cc,
	}
}
