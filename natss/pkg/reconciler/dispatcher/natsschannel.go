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
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/tools/cache"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	"knative.dev/eventing/pkg/logging"
	"knative.dev/pkg/controller"

	"knative.dev/eventing-contrib/natss/pkg/apis/messaging/v1alpha1"
	messaginginformers "knative.dev/eventing-contrib/natss/pkg/client/informers/externalversions/messaging/v1alpha1"
	listers "knative.dev/eventing-contrib/natss/pkg/client/listers/messaging/v1alpha1"
	"knative.dev/eventing-contrib/natss/pkg/dispatcher"
	"knative.dev/eventing-contrib/natss/pkg/reconciler"
)

const (
	// ReconcilerName is the name of the reconciler.
	ReconcilerName = "NatssChannels"

	// controllerAgentName is the string used by this controller to identify
	// itself when creating events.
	controllerAgentName = "natss-ch-dispatcher"

	finalizerName = controllerAgentName
)

// Reconciler reconciles NATSS Channels.
type Reconciler struct {
	*reconciler.Base

	natssDispatcher dispatcher.NatssDispatcher

	natsschannelLister listers.NatssChannelLister
	impl               *controller.Impl
}

// Check that our Reconciler implements controller.Reconciler.
var _ controller.Reconciler = (*Reconciler)(nil)

// NewController initializes the controller and is called by the generated code.
// Registers event handlers to enqueue events.
func NewController(
	opt reconciler.Options,
	natssDispatcher dispatcher.NatssDispatcher,
	natsschannelInformer messaginginformers.NatssChannelInformer,
) *controller.Impl {

	r := &Reconciler{
		Base:               reconciler.NewBase(opt, controllerAgentName),
		natssDispatcher:    natssDispatcher,
		natsschannelLister: natsschannelInformer.Lister(),
	}
	r.impl = controller.NewImpl(r, r.Logger, ReconcilerName)

	r.Logger.Info("Setting up event handlers")

	// Watch for NATSS channels.
	natsschannelInformer.Informer().AddEventHandler(controller.HandleAll(r.impl.Enqueue))

	return r.impl
}

// Reconcile a NatssChannel and perform the following actions:
// - update natss subscriptions based on NatssChannel subscribers
// - set NatssChannel subscriber status based on natss subscriptions
// - maintain mapping between host header and natss channel which is used by the dispatcher
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	// Convert the namespace/name string into a distinct namespace and name.
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logging.FromContext(ctx).Error("invalid resource key")
		return nil
	}

	// Get the NatssChannel resource with this namespace/name.
	original, err := r.natsschannelLister.NatssChannels(namespace).Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logging.FromContext(ctx).Error("NatssChannel key in work queue no longer exists")
		return nil
	} else if err != nil {
		return err
	}

	// Don't modify the informers copy.
	natssChannel := original.DeepCopy()

	// See if the channel has been deleted.
	if natssChannel.DeletionTimestamp != nil {
		c := toChannel(natssChannel)

		if _, err := r.natssDispatcher.UpdateSubscriptions(c, true); err != nil {
			logging.FromContext(ctx).Error("Error updating subscriptions", zap.Any("channel", c), zap.Error(err))
			return err
		}
		removeFinalizer(natssChannel)
		_, err := r.NatssClientSet.MessagingV1alpha1().NatssChannels(natssChannel.Namespace).Update(natssChannel)
		return err
	}

	if !natssChannel.Status.IsReady() {
		return fmt.Errorf("Channel is not ready. Cannot configure and update subscriber status")
	}

	reconcileErr := r.reconcile(ctx, natssChannel)
	if reconcileErr != nil {
		logging.FromContext(ctx).Error("Error reconciling NatssChannel", zap.Error(reconcileErr))
	} else {
		logging.FromContext(ctx).Debug("NatssChannel reconciled")
	}

	// TODO: Should this check for subscribable status rather than entire status?
	if _, updateStatusErr := r.updateStatus(ctx, natssChannel); updateStatusErr != nil {
		logging.FromContext(ctx).Error("Failed to update NatssChannel status", zap.Error(updateStatusErr))
		return updateStatusErr
	}

	// Trigger reconciliation in case of errors
	return reconcileErr
}

// reconcile performs the following steps
// - add finalizer
// - update natss subscriptions
// - set NatssChannel SubscribableStatus
// - update host2channel map
func (r *Reconciler) reconcile(ctx context.Context, natssChannel *v1alpha1.NatssChannel) error {
	// TODO update dispatcher API and use Channelable or NatssChannel.
	c := toChannel(natssChannel)

	// If we are adding the finalizer for the first time, then ensure that finalizer is persisted
	// before manipulating Natss.
	if err := r.ensureFinalizer(natssChannel); err != nil {
		return err
	}

	// Try to subscribe.
	failedSubscriptions, err := r.natssDispatcher.UpdateSubscriptions(c, false)
	if err != nil {
		logging.FromContext(ctx).Error("Error updating subscriptions", zap.Any("channel", c), zap.Error(err))
		return err
	}

	natssChannel.Status.SubscribableStatus = r.createSubscribableStatus(natssChannel.Spec.Subscribable, failedSubscriptions)
	if len(failedSubscriptions) > 0 {
		var b strings.Builder
		for _, subError := range failedSubscriptions {
			b.WriteString("\n")
			b.WriteString(subError.Error())
		}
		errMsg := b.String()
		logging.FromContext(ctx).Error(errMsg)
		return fmt.Errorf(errMsg)
	}

	natssChannels, err := r.natsschannelLister.List(labels.Everything())
	if err != nil {
		logging.FromContext(ctx).Error("Error listing natss channels")
		return err
	}

	channels := make([]messagingv1alpha1.Channel, 0)
	for _, nc := range natssChannels {
		if nc.Status.IsReady() {
			channels = append(channels, *toChannel(nc))
		}
	}

	if err := r.natssDispatcher.ProcessChannels(ctx, channels); err != nil {
		logging.FromContext(ctx).Error("Error updating host to channel map", zap.Error(err))
		return err
	}

	return nil
}

// createSubscribableStatus creates the SubscribableStatus based on the failedSubscriptions
// checks for each subscriber on the natss channel if there is a failed subscription on natss side
// if there is no failed subscription => set ready status
func (r *Reconciler) createSubscribableStatus(subscribable *eventingduck.Subscribable, failedSubscriptions map[eventingduck.SubscriberSpec]error) *eventingduck.SubscribableStatus {
	if subscribable == nil {
		return nil
	}
	subscriberStatus := make([]eventingduck.SubscriberStatus, 0)
	for _, sub := range subscribable.Subscribers {
		status := eventingduck.SubscriberStatus{
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
	return &eventingduck.SubscribableStatus{
		Subscribers: subscriberStatus,
	}
}

func (r *Reconciler) updateStatus(ctx context.Context, desired *v1alpha1.NatssChannel) (*v1alpha1.NatssChannel, error) {
	nc, err := r.natsschannelLister.NatssChannels(desired.Namespace).Get(desired.Name)
	if err != nil {
		return nil, err
	}

	if reflect.DeepEqual(nc.Status, desired.Status) {
		return nc, nil
	}

	// Don't modify the informers copy.
	existing := nc.DeepCopy()
	existing.Status = desired.Status

	return r.NatssClientSet.MessagingV1alpha1().NatssChannels(desired.Namespace).UpdateStatus(existing)
}

func (r *Reconciler) ensureFinalizer(channel *v1alpha1.NatssChannel) error {
	finalizers := sets.NewString(channel.Finalizers...)
	if finalizers.Has(finalizerName) {
		return nil
	}

	mergePatch := map[string]interface{}{
		"metadata": map[string]interface{}{
			"finalizers":      append(channel.Finalizers, finalizerName),
			"resourceVersion": channel.ResourceVersion,
		},
	}

	patch, err := json.Marshal(mergePatch)
	if err != nil {
		return err
	}

	_, err = r.NatssClientSet.MessagingV1alpha1().NatssChannels(channel.Namespace).Patch(channel.Name, types.MergePatchType, patch)
	return err
}

func removeFinalizer(channel *v1alpha1.NatssChannel) {
	finalizers := sets.NewString(channel.Finalizers...)
	finalizers.Delete(finalizerName)
	channel.Finalizers = finalizers.List()
}

func toChannel(natssChannel *v1alpha1.NatssChannel) *messagingv1alpha1.Channel {
	channel := &messagingv1alpha1.Channel{
		ObjectMeta: v1.ObjectMeta{
			Name:      natssChannel.Name,
			Namespace: natssChannel.Namespace,
		},
		Spec: messagingv1alpha1.ChannelSpec{
			Subscribable: natssChannel.Spec.Subscribable,
		},
	}
	if natssChannel.Status.Address != nil {
		channel.Status = messagingv1alpha1.ChannelStatus{
			AddressStatus: natssChannel.Status.AddressStatus,
		}
	}
	return channel
}
