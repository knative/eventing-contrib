package controller

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	"knative.dev/pkg/controller"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"

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
			Name: "remove finalizer even though channel is not ready",
			Key:  ncKey,
			Objects: []runtime.Object{
				reconciletesting.NewNatssChannel(ncName, testNS,
					// the finalizer should get removed
					reconciletesting.WithNatssChannelFinalizer,
					// make sure that finalizer can get removed even if channel is not ready
					reconciletesting.WithNatssChannelServicetNotReady("ChannelServiceFailed", "Channel Service failed: services \"default-kne-ingress-kn-channel\" is forbidden: unable to create new content in namespace e2e-mesh-ns because it is being terminated"),
					reconciletesting.WithNatssChannelDeleted),
			},
			WantErr: false,
			WantUpdates: []clientgotesting.UpdateActionImpl{
				{
					Object: reconciletesting.NewNatssChannel(ncName, testNS,
						// finalizer is removed
						// make sure that finalizer can get removed even if channel is not ready
						reconciletesting.WithNatssChannelServicetNotReady("ChannelServiceFailed", "Channel Service failed: services \"default-kne-ingress-kn-channel\" is forbidden: unable to create new content in namespace e2e-mesh-ns because it is being terminated"),
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
