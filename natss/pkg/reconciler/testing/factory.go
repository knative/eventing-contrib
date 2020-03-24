/*
Copyright 2019 The Knative Authors.

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

package testing

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"

	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/client-go/tools/record"

	fakeclientset "knative.dev/eventing-contrib/natss/pkg/client/injection/client/fake"

	fakeeventingclient "knative.dev/eventing/pkg/client/injection/client/fake"

	fakekubeclient "knative.dev/pkg/client/injection/kube/client/fake"
	"knative.dev/pkg/controller"
	fakedynamicclient "knative.dev/pkg/injection/clients/dynamicclient/fake"
	"knative.dev/pkg/logging"
	logtesting "knative.dev/pkg/logging/testing"
	. "knative.dev/pkg/reconciler/testing"
)

const (
	// maxEventBufferSize is the estimated max number of event notifications that
	// can be buffered during reconciliation.
	maxEventBufferSize = 10
)

// Ctor functions create a k8s controller with given params.
type Ctor func(context.Context, *Listers) controller.Reconciler

// makeFactory creates a reconciler factory with fake clients and controller created by `ctor`.
func makeFactory(ctor Ctor) Factory {
	return func(t *testing.T, r *TableRow) (controller.Reconciler, ActionRecorderList, EventList) {
		ls := NewListers(r.Objects)

		ctx := logging.WithLogger(context.Background(), logtesting.TestLogger(t))
		ctx, kubeClient := fakekubeclient.With(ctx, ls.GetKubeObjects()...)
		ctx, eventingClient := fakeeventingclient.With(ctx, ls.GetEventingObjects()...)
		ctx, client := fakeclientset.With(ctx, ls.GetMessagingObjects()...)

		dynamicScheme := runtime.NewScheme()
		for _, addTo := range clientSetSchemes {
			addTo(dynamicScheme)
		}

		ctx, dynamicClient := fakedynamicclient.With(ctx, dynamicScheme, ls.GetAllObjects()...)
		eventRecorder := record.NewFakeRecorder(maxEventBufferSize)

		// Set up our Controller from the fakes.

		for _, reactor := range r.WithReactors {
			kubeClient.PrependReactor("*", "*", reactor)
			eventingClient.PrependReactor("*", "*", reactor)
			client.PrependReactor("*", "*", reactor)
			dynamicClient.PrependReactor("*", "*", reactor)
		}

		// Validate all Create operations through the eventing client.
		client.PrependReactor("create", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			return ValidateCreates(context.Background(), action)
		})
		client.PrependReactor("update", "*", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			return ValidateUpdates(context.Background(), action)
		})

		actionRecorderList := ActionRecorderList{dynamicClient, client, kubeClient}
		eventList := EventList{Recorder: eventRecorder}

		ctx = controller.WithEventRecorder(ctx, eventRecorder)

		c := ctor(ctx, &ls)
		return c, actionRecorderList, eventList
	}
}

func MakeFactoryWithContext(ctor func(context.Context, *Listers) controller.Reconciler) Factory {
	return makeFactory(ctor)
}
