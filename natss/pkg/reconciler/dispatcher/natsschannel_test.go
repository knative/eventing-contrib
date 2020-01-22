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
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"
	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"

	clientset "knative.dev/eventing-contrib/natss/pkg/client/clientset/versioned"
	informers "knative.dev/eventing-contrib/natss/pkg/client/informers/externalversions"
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
			Name: "make sure reconcile handles bad keys",
			Key:  "too/many/parts",
		},
		{
			Name: "make sure reconcile handles good keys that don't exist",
			Key:  "foo/not-found",
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
					reconciletesting.WithNotReady("ups", ""),
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
					reconciletesting.WithNotReady("ups", ""),
					reconciletesting.WithNatssChannelDeleted),
			},
			WantErr: false,
			WantUpdates: []clientgotesting.UpdateActionImpl{
				{
					Object: reconciletesting.NewNatssChannel(ncName, testNS,
						// finalizer is removed
						// make sure that finalizer can get removed even if channel is not ready
						reconciletesting.WithNotReady("ups", ""),
						reconciletesting.WithNatssChannelDeleted),
				},
			},
		},
	}
	defer logtesting.ClearAll()

	table.Test(t, reconciletesting.MakeFactory(func(listers *reconciletesting.Listers, opt reconciler.Options) controller.Reconciler {
		natssDispatcher := dispatchertesting.NewDispatcherDoNothing()

		r := Reconciler{
			Base:               reconciler.NewBase(opt, controllerAgentName),
			natssDispatcher:    natssDispatcher,
			natsschannelLister: listers.GetNatssChannelLister(),
			impl:               nil,
		}

		return &r
	},
	))
}
func TestFailedNatssSubscription(t *testing.T) {
	ncKey := testNS + "/" + ncName

	table := TableTest{
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
		natssDispatcher := dispatchertesting.NewDispatcherFailNatssSubscription()

		r := Reconciler{
			Base:               reconciler.NewBase(opt, controllerAgentName),
			natssDispatcher:    natssDispatcher,
			natsschannelLister: listers.GetNatssChannelLister(),
			impl:               nil,
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
	natssChannelInformer := messagingInformerFactory.Messaging().V1alpha1().NatssChannels()

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
	}, dispatchertesting.NewDispatcherDoNothing(), natssChannelInformer)
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
