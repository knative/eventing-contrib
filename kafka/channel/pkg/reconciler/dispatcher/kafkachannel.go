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
	"reflect"

	"go.uber.org/zap"

	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"

	eventingduckv1alpha1 "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	eventingduckv1beta1 "knative.dev/eventing/pkg/apis/duck/v1beta1"
	"knative.dev/eventing/pkg/apis/eventing"
	"knative.dev/eventing/pkg/channel/fanout"
	"knative.dev/eventing/pkg/channel/multichannelfanout"
	"knative.dev/eventing/pkg/kncloudevents"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	pkgreconciler "knative.dev/pkg/reconciler"

	"knative.dev/eventing-contrib/kafka/channel/pkg/apis/messaging/v1alpha1"
	kafkaclientset "knative.dev/eventing-contrib/kafka/channel/pkg/client/clientset/versioned"
	kafkaScheme "knative.dev/eventing-contrib/kafka/channel/pkg/client/clientset/versioned/scheme"
	kafkaclientsetinjection "knative.dev/eventing-contrib/kafka/channel/pkg/client/injection/client"
	"knative.dev/eventing-contrib/kafka/channel/pkg/client/injection/informers/messaging/v1alpha1/kafkachannel"
	listers "knative.dev/eventing-contrib/kafka/channel/pkg/client/listers/messaging/v1alpha1"
	"knative.dev/eventing-contrib/kafka/channel/pkg/dispatcher"
	"knative.dev/eventing-contrib/kafka/channel/pkg/utils"
)

func init() {
	// Add run types to the default Kubernetes Scheme so Events can be
	// logged for run types.
	_ = kafkaScheme.AddToScheme(scheme.Scheme)
}

const (
	// ReconcilerName is the name of the reconciler.
	ReconcilerName = "KafkaChannels"

	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "kafka-ch-dispatcher"

	// Name of the corev1.Events emitted from the reconciliation process.
	channelReconciled         = "ChannelReconciled"
	channelReconcileFailed    = "ChannelReconcileFailed"
	channelUpdateStatusFailed = "ChannelUpdateStatusFailed"
)

// Reconciler reconciles Kafka Channels.
type Reconciler struct {
	recorder record.EventRecorder

	kafkaDispatcher *dispatcher.KafkaDispatcher

	kafkaClientSet       kafkaclientset.Interface
	kafkachannelLister   listers.KafkaChannelLister
	kafkachannelInformer cache.SharedIndexInformer
	impl                 *controller.Impl
}

// Check that our Reconciler implements controller.Reconciler.
var _ controller.Reconciler = (*Reconciler)(nil)

// NewController initializes the controller and is called by the generated code.
// Registers event handlers to enqueue events.
func NewController(ctx context.Context, _ configmap.Watcher) *controller.Impl {

	logger := logging.FromContext(ctx)

	configMap, err := configmap.Load("/etc/config-kafka")
	if err != nil {
		logger.Fatal("error loading configuration", zap.Error(err))
	}

	kafkaConfig, err := utils.GetKafkaConfig(configMap)
	if err != nil {
		logger.Fatal("Error loading kafka config", zap.Error(err))
	}

	connectionArgs := &kncloudevents.ConnectionArgs{
		MaxIdleConns:        kafkaConfig.MaxIdleConns,
		MaxIdleConnsPerHost: kafkaConfig.MaxIdleConnsPerHost,
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
		logger.Fatal("Unable to create kafka dispatcher", zap.Error(err))
	}
	logger.Info("Starting the Kafka dispatcher")
	logger.Info("Kafka broker configuration", zap.Strings(utils.BrokerConfigMapKey, kafkaConfig.Brokers))

	r := &Reconciler{
		recorder:             getRecorder(ctx, injection.GetConfig(ctx)),
		kafkaDispatcher:      kafkaDispatcher,
		kafkaClientSet:       kafkaclientsetinjection.Get(ctx),
		kafkachannelLister:   kafkaChannelInformer.Lister(),
		kafkachannelInformer: kafkaChannelInformer.Informer(),
	}
	r.impl = controller.NewImpl(r, logger.Sugar(), ReconcilerName)

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
			logger.Error("Cannot start dispatcher", zap.Error(err))
		}
	}()

	return r.impl
}

func getRecorder(ctx context.Context, cfg *rest.Config) record.EventRecorder {
	recorder := controller.GetEventRecorder(ctx)
	if recorder == nil {
		// Create event broadcaster
		logging.FromContext(ctx).Debug("Creating event broadcaster")
		eventBroadcaster := record.NewBroadcaster()
		watches := []watch.Interface{
			eventBroadcaster.StartLogging(logging.FromContext(ctx).Sugar().Named("event-broadcaster").Infof),
			eventBroadcaster.StartRecordingToSink(
				&typedcorev1.EventSinkImpl{Interface: kubernetes.NewForConfigOrDie(cfg).CoreV1().Events("")}),
		}
		recorder = eventBroadcaster.NewRecorder(
			scheme.Scheme, corev1.EventSource{Component: controllerAgentName})
		go func() {
			<-ctx.Done()
			for _, w := range watches {
				w.Stop()
			}
		}()
	}
	return recorder
}

func filterWithAnnotation(namespaced bool) func(obj interface{}) bool {
	if namespaced {
		return pkgreconciler.AnnotationFilterFunc(eventing.ScopeAnnotationKey, "namespace", false)
	}
	return pkgreconciler.AnnotationFilterFunc(eventing.ScopeAnnotationKey, "cluster", true)
}

func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name.
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Error("invalid resource key")
		return nil
	}

	// Get the KafkaChannel resource with this namespace/name.
	original, err := r.kafkachannelLister.KafkaChannels(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Error("KafkaChannel key in work queue no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	if !original.Status.IsReady() {
		return fmt.Errorf("Channel is not ready. Cannot configure and update subscriber status")
	}

	// Don't modify the informers copy.
	channel := original.DeepCopy()

	// Reconcile this copy of the KafkaChannel and then write back any status updates regardless of
	// whether the reconcile error out.
	reconcileErr := r.reconcile(ctx, channel)
	if reconcileErr != nil {
		logging.FromContext(ctx).Error("Error reconciling KafkaChannel", zap.Error(reconcileErr))
		r.recorder.Eventf(channel, corev1.EventTypeWarning, channelReconcileFailed, "KafkaChannel reconciliation failed: %v", reconcileErr)
	} else {
		logging.FromContext(ctx).Debug("KafkaChannel reconciled")
		r.recorder.Event(channel, corev1.EventTypeNormal, channelReconciled, "KafkaChannel reconciled")
	}

	// TODO: Should this check for subscribable status rather than entire status?
	if _, updateStatusErr := r.updateStatus(ctx, channel); updateStatusErr != nil {
		logging.FromContext(ctx).Error("Failed to update KafkaChannel status", zap.Error(updateStatusErr))
		r.recorder.Eventf(channel, corev1.EventTypeWarning, channelUpdateStatusFailed, "Failed to update KafkaChannel's status: %v", updateStatusErr)
		return updateStatusErr
	}

	// Requeue if the resource is not ready
	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, kc *v1alpha1.KafkaChannel) error {
	channels, err := r.kafkachannelLister.List(labels.Everything())
	if err != nil {
		logging.FromContext(ctx).Error("Error listing kafka channels")
		return err
	}

	// TODO: revisit this code. Instead of reading all channels and updating consumers and hostToChannel map for all
	// why not just reconcile the current channel. With this the UpdateKafkaConsumers can now return SubscribableStatus
	// for the subscriptions on the channel that is being reconciled.
	kafkaChannels := make([]*v1alpha1.KafkaChannel, 0)
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

	failedSubscriptionsv1beta1, err := r.kafkaDispatcher.UpdateKafkaConsumers(config)
	if err != nil {
		logging.FromContext(ctx).Error("Error updating kafka consumers in dispatcher")
		return err
	}
	failedSubscriptionsv1alpha1 := make(map[eventingduckv1alpha1.SubscriberSpec]error, len(failedSubscriptionsv1beta1))
	for k, v := range failedSubscriptionsv1beta1 {
		newSub := eventingduckv1alpha1.SubscriberSpec{}
		newSub.ConvertFrom(context.TODO(), k)
		failedSubscriptionsv1alpha1[newSub] = v
	}
	kc.Status.SubscribableTypeStatus.SubscribableStatus = r.createSubscribableStatus(kc.Spec.Subscribable, failedSubscriptionsv1alpha1)
	if len(failedSubscriptionsv1alpha1) > 0 {
		logging.FromContext(ctx).Error("Some kafka subscriptions failed to subscribe")
		return fmt.Errorf("Some kafka subscriptions failed to subscribe")
	}
	return nil
}

func (r *Reconciler) createSubscribableStatus(subscribable *eventingduckv1alpha1.Subscribable, failedSubscriptions map[eventingduckv1alpha1.SubscriberSpec]error) *eventingduckv1alpha1.SubscribableStatus {
	if subscribable == nil {
		return nil
	}
	subscriberStatus := make([]eventingduckv1alpha1.SubscriberStatus, 0)
	for _, sub := range subscribable.Subscribers {
		status := eventingduckv1alpha1.SubscriberStatus{
			UID:                sub.UID,
			ObservedGeneration: sub.Generation,
			Ready:              corev1.ConditionTrue,
		}
		if err, ok := failedSubscriptions[sub]; ok {
			status.Ready = corev1.ConditionFalse
			status.Message = err.Error()
		}
		subscriberStatus = append(subscriberStatus, status)
	}
	return &eventingduckv1alpha1.SubscribableStatus{
		Subscribers: subscriberStatus,
	}
}

// newConfigFromKafkaChannels creates a new Config from the list of kafka channels.
func (r *Reconciler) newChannelConfigFromKafkaChannel(c *v1alpha1.KafkaChannel) *multichannelfanout.ChannelConfig {
	channelConfig := multichannelfanout.ChannelConfig{
		Namespace: c.Namespace,
		Name:      c.Name,
		HostName:  c.Status.Address.Hostname,
	}
	if c.Spec.Subscribable != nil {
		newSubs := make([]eventingduckv1beta1.SubscriberSpec, len(c.Spec.Subscribable.Subscribers))
		for i, source := range c.Spec.Subscribable.Subscribers {
			source.ConvertTo(context.TODO(), &newSubs[i])
			fmt.Printf("Converted \n%+v\n To\n%+v\n", source, newSubs[i])
			fmt.Printf("Delivery converted \n%+v\nto\n%+v\n", source.Delivery, newSubs[i].Delivery)
		}
		channelConfig.FanoutConfig = fanout.Config{
			AsyncHandler:  true,
			Subscriptions: newSubs,
		}
	}
	return &channelConfig
}

// newConfigFromKafkaChannels creates a new Config from the list of kafka channels.
func (r *Reconciler) newConfigFromKafkaChannels(channels []*v1alpha1.KafkaChannel) *multichannelfanout.Config {
	cc := make([]multichannelfanout.ChannelConfig, 0)
	for _, c := range channels {
		channelConfig := r.newChannelConfigFromKafkaChannel(c)
		cc = append(cc, *channelConfig)
	}
	return &multichannelfanout.Config{
		ChannelConfigs: cc,
	}
}
func (r *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.KafkaChannel) (*v1alpha1.KafkaChannel, error) {
	kc, err := r.kafkachannelLister.KafkaChannels(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}

	if reflect.DeepEqual(kc.Status, desired.Status) {
		return kc, nil
	}

	// Don't modify the informers copy.
	existing := kc.DeepCopy()
	existing.Status = desired.Status

	new, err := r.kafkaClientSet.MessagingV1alpha1().KafkaChannels(desired.Namespace).UpdateStatus(existing)
	return new, err
}
