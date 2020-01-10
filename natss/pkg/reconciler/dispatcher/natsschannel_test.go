package controller

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	eventingduck "knative.dev/eventing/pkg/apis/duck/v1alpha1"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"

	"knative.dev/eventing-contrib/natss/pkg/dispatcher"
	"knative.dev/eventing-contrib/natss/pkg/reconciler"
	reconciletesting "knative.dev/eventing-contrib/natss/pkg/reconciler/testing"
)

const (
	systemNS                 = "knative-eventing"
	testNS                   = "test-namespace"
	ncName                   = "test-nc"
	dispatcherDeploymentName = "test-deployment"
	dispatcherServiceName    = "test-service"
	channelServiceAddress    = "test-nc-kn-channel.test-namespace.svc.cluster.local"
)

func TestAllCases(t *testing.T) {
	ncKey := testNS + "/" + ncName
	ncSubscriber := "http://foo.bar"
	ncSubscriberURL, err := apis.ParseURL(ncSubscriber)

	if err != nil {
		t.Fatalf("cannot parse url: %+v", err)
	}
	nC := reconciletesting.NewNatssChannel(ncName, testNS,
		// TODO(nachtmaar): remove ?
		reconciletesting.WithNatssInitChannelConditions,
		// make sure that finalizer can get removed even if channel is not ready
		reconciletesting.WithNatssChannelServicetNotReady("ChannelServiceFailed", "Channel Service failed: services \"default-kne-ingress-kn-channel\" is forbidden: unable to create new content in namespace e2e-mesh-ns because it is being terminated"),
	)
	nC.Spec.Subscribable = &eventingduck.Subscribable{
		// this field needs to be non-nil to update the internal subscriptions map
		Subscribers: []eventingduck.SubscriberSpec{
			{
				UID:               "",
				Generation:        0,
				SubscriberURI:     ncSubscriberURL,
			},
		},
	}
	table := TableTest{
		{
			Name: "remove finalizer even though channel is not ready",
			Key:  ncKey,
			Objects: []runtime.Object{
				// channel which is not marked for deletion yet
				// required to populate the subscriptions map inside the natssdispatcher reconciler
				// channel is marked for deletion now
				reconciletesting.NewNatssChannel(ncName, testNS,
					// TODO(nachtmaar): remove ?
					reconciletesting.WithNatssInitChannelConditions,
					// make sure that finalizer can get removed even if channel is not ready
					reconciletesting.WithNatssChannelServicetNotReady("ChannelServiceFailed", "Channel Service failed: services \"default-kne-ingress-kn-channel\" is forbidden: unable to create new content in namespace e2e-mesh-ns because it is being terminated"),
					reconciletesting.WithNatssChannelDeleted),
			},
			WithReactors: []clientgotesting.ReactionFunc{
				// TODO(nachtmaar): only for natsschannels
				//PrepareSubscriptionsMap("*", "*", nC),
				InduceFailure("*", "*"),
			},
			WantErr: false,
			WantEvents: []string{
				// TODO(nachtmaar):
			},
		},
	}
	defer logtesting.ClearAll()

	table.Test(t, reconciletesting.MakeFactory(func(listers *reconciletesting.Listers, opt reconciler.Options) controller.Reconciler {
		//subscriptions := make(dispatcher.SubscriptionChannelMapping)
		// map[eventingchannels.ChannelReference]map[subscriptionReference]*stan.Subscription
		//chRef := channel.ChannelReference{
		//	Namespace: testNS,
		//	Name:      ncName,
		//}
		//subscriptions[chRef] = map[dispatcher.Sub]
		natssDispatcher, err := dispatcher.NewTestingDispatcher(t, nil)
		// TODO(nachtmaar)
		if err != nil {
			panic(err)
		}

		//natssChannel := reconciletesting.NewNatssChannel(ncName, testNS,
		//	// TODO(nachtmaar): remove ?
		//	reconciletesting.WithNatssInitChannelConditions,
		//	// make sure that finalizer can get removed even if channel is not ready
		//	reconciletesting.WithNatssChannelServicetNotReady("ChannelServiceFailed", "Channel Service failed: services \"default-kne-ingress-kn-channel\" is forbidden: unable to create new content in namespace e2e-mesh-ns because it is being terminated"),
		//)

		reconciler := Reconciler{
			Base:               reconciler.NewBase(opt, controllerAgentName),
			natssDispatcher:    natssDispatcher,
			natsschannelLister: listers.GetNatssChannelLister(),
			// TODO fix
			natsschannelInformer: nil,
			impl:                 nil,
		}
		if failedSubscriptions, err := natssDispatcher.UpdateSubscriptions(toChannel(nC), false); err != nil || len(failedSubscriptions) > 0 {
			t.Fatalf("unable to prepare subscriptions state: failedSubscriptions: %+v, error: %+v", failedSubscriptions, err)
		}

		return &reconciler
	},
	))
}

func PrepareSubscriptionsMap(verb, resource string, obj runtime.Object) clientgotesting.ReactionFunc {
	return func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		if !action.Matches(verb, resource) {
			return false, nil, nil
		}
		return true, obj, nil
	}
}
