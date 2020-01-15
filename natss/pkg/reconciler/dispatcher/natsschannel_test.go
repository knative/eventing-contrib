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

package controller

import (
	"context"
	"errors"
	"testing"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	clientgotesting "k8s.io/client-go/testing"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	messagingv1alpha1 "knative.dev/eventing/pkg/apis/messaging/v1alpha1"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"

	clientset "knative.dev/eventing-contrib/natss/pkg/client/clientset/versioned"
	informers "knative.dev/eventing-contrib/natss/pkg/client/informers/externalversions"
	"knative.dev/eventing-contrib/natss/pkg/dispatcher"
	dispatchertesting "knative.dev/eventing-contrib/natss/pkg/dispatcher/testing"
	"knative.dev/eventing-contrib/natss/pkg/reconciler"
	reconciletesting "knative.dev/eventing-contrib/natss/pkg/reconciler/testing"
)

const (
	testNS = "test-namespace"
	ncName = "test-nc"
)

func TestAllCases(t *testing.T) {
	ncKey := testNS + "/" + ncName

	table := TableTest{
		{
			Name: "bad workqueue key",
			// Make sure Reconcile handles bad keys.
			Key: "too/many/parts",
		},
		{
			Name: "key not found",
			// Make sure Reconcile handles good keys that don't exist.
			Key: "foo/not-found",
		},
		{
			Name: "reconcile ok: channel ready",
			Key:  ncKey,
			Objects: []runtime.Object{
				reconciletesting.NewNatssChannel(ncName, testNS,
					reconciletesting.WithReady,
				),
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				makeFinalizerPatch(testNS, ncName),
			},
			WantErr: false,
		},
		{
			Name: "reconcile error: channel not ready",
			Key:  ncKey,
			Objects: []runtime.Object{
				reconciletesting.NewNatssChannel(ncName, testNS,
					reconciletesting.WithNatssChannelServiceNotReady("ChannelServiceFailed", "some message"),
				),
			},
			WantErr: true,
		},
		{
			Name: "remove finalizer even though channel is not ready",
			Key:  ncKey,
			Objects: []runtime.Object{
				reconciletesting.NewNatssChannel(ncName, testNS,
					// the finalizer should get removed
					reconciletesting.WithNatssChannelFinalizer,
					// make sure that finalizer can get removed even if channel is not ready
					reconciletesting.WithNatssChannelServiceNotReady("ChannelServiceFailed", "Channel Service failed: services \"default-kne-ingress-kn-channel\" is forbidden: unable to create new content in namespace e2e-mesh-ns because it is being terminated"),
					reconciletesting.WithNatssChannelDeleted),
			},
			WantErr: false,
			WantUpdates: []clientgotesting.UpdateActionImpl{
				{
					Object: reconciletesting.NewNatssChannel(ncName, testNS,
						// finalizer is removed
						// make sure that finalizer can get removed even if channel is not ready
						reconciletesting.WithNatssChannelServiceNotReady("ChannelServiceFailed", "Channel Service failed: services \"default-kne-ingress-kn-channel\" is forbidden: unable to create new content in namespace e2e-mesh-ns because it is being terminated"),
						reconciletesting.WithNatssChannelDeleted),
				},
			},
		},
	}
	defer logtesting.ClearAll()

	table.Test(t, reconciletesting.MakeFactory(func(listers *reconciletesting.Listers, opt reconciler.Options) controller.Reconciler {
		natssDispatcher := dispatchertesting.NewDispatcher(t)

		r := Reconciler{
			Base:                 reconciler.NewBase(opt, controllerAgentName),
			natssDispatcher:      natssDispatcher,
			natsschannelLister:   listers.GetNatssChannelLister(),
			natsschannelInformer: nil,
			impl:                 nil,
		}

		return &r
	},
	))
}
func TestBrokenUpdate(t *testing.T) {
	ncKey := testNS + "/" + ncName

	table := TableTest{
		// test that a
		{
			Name: "a failed natss subscription is reflected in Status.SubscribableStatus",
			Objects: []runtime.Object{
				reconciletesting.NewNatssChannel(ncName, testNS,
					reconciletesting.WithReady,
					// add subscriber for channel
					reconciletesting.WithNatssChannelSubscribers(t, "http://dummy.org"),
				),
			},
			Key: ncKey,
			WantStatusUpdates: []clientgotesting.UpdateActionImpl{
				{
					Object: reconciletesting.NewNatssChannel(ncName, testNS,
						reconciletesting.WithReady,
						// add subscriber for channel
						reconciletesting.WithNatssChannelSubscribers(t, "http://dummy.org"),
						// status of subscriber should be not ready, because SubscriptionsSupervisorUpdateBroken simulates a failed natss subscription
						reconciletesting.WithNatssChannelSubscribableStatus(corev1.ConditionFalse, "ups"),
					),
				},
			},
			WantPatches: []clientgotesting.PatchActionImpl{
				makeFinalizerPatch(testNS, ncName),
			},
			WantErr: true,
		},
	}
	defer logtesting.ClearAll()

	table.Test(t, reconciletesting.MakeFactory(func(listers *reconciletesting.Listers, opt reconciler.Options) controller.Reconciler {
		natssDispatcher := NewSubscriptionsSupervisorUpdateBroken(t)

		r := Reconciler{
			Base:                 reconciler.NewBase(opt, controllerAgentName),
			natssDispatcher:      natssDispatcher,
			natsschannelLister:   listers.GetNatssChannelLister(),
			natsschannelInformer: nil,
			impl:                 nil,
		}

		return &r
	},
	))
}

// Test that the dispatcher controller can be created
func TestNewController(t *testing.T) {
	messagingClientSet, err := clientset.NewForConfig(&rest.Config{})
	if err != nil {
		t.Fail()
	}

	messagingInformerFactory := informers.NewSharedInformerFactory(messagingClientSet, 0)
	y := messagingInformerFactory.Messaging().V1alpha1().NatssChannels()

	c := NewController(reconciler.Options{
		KubeClientSet:    fakekubeclientset.NewSimpleClientset(),
		DynamicClientSet: nil,
		NatssClientSet:   nil,
		Recorder:         nil,
		StatsReporter:    nil,
		ConfigMapWatcher: nil,
		Logger:           logtesting.TestLogger(t),
		ResyncPeriod:     0,
		StopChannel:      nil,
	}, dispatchertesting.NewDispatcher(t), y)
	if c == nil {
		t.Errorf("unable to create dispatcher controller")
	}
}

func makeFinalizerPatch(namespace, name string) clientgotesting.PatchActionImpl {
	action := clientgotesting.PatchActionImpl{}
	action.Name = name
	action.Namespace = namespace
	patch := `{"metadata":{"finalizers":["` + finalizerName + `"],"resourceVersion":""}}`
	action.Patch = []byte(patch)
	return action
}

type FailNatssSubscriptionSimulator struct {
	logger *zap.Logger
}

var _ dispatcher.NatssDispatcher = (*FailNatssSubscriptionSimulator)(nil)

func NewSubscriptionsSupervisorUpdateBroken(t *testing.T) *FailNatssSubscriptionSimulator {
	return &FailNatssSubscriptionSimulator{logger: logtesting.TestLogger(t).Desugar()}
}

func (s *FailNatssSubscriptionSimulator) Start(_ <-chan struct{}) error {
	s.logger.Info("start")
	return errors.New("ups")
}

func (s *FailNatssSubscriptionSimulator) UpdateSubscriptions(channel *messagingv1alpha1.Channel, _ bool) (map[eventingduck.SubscriberSpec]error, error) {
	s.logger.Info("updating subscriptions")
	failedSubscriptions := make(map[eventingduck.SubscriberSpec]error, 0)
	for _, sub := range channel.Spec.Subscribable.Subscribers {
		failedSubscriptions[sub] = errors.New("ups")
	}
	return failedSubscriptions, nil
}

func (s *FailNatssSubscriptionSimulator) UpdateHostToChannelMap(_ context.Context, _ []messagingv1alpha1.Channel) error {
	s.logger.Info("updating hosttochannel map")
	return errors.New("ups")
}
