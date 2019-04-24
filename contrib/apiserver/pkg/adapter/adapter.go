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

package adapter

import (
	"context"
	"fmt"
	"os"
	"strconv"

	"github.com/cloudevents/sdk-go/pkg/cloudevents"
	eventsclient "github.com/cloudevents/sdk-go/pkg/cloudevents/client"
	"github.com/cloudevents/sdk-go/pkg/cloudevents/types"
	"github.com/knative/eventing-sources/pkg/kncloudevents"
	"github.com/knative/pkg/logging"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

const (
	controllerAgentName = "resource-controller-adapter"
	updateEventType     = "dev.knative.apiserver.object.update"
	deleteEventType     = "dev.knative.apiserver.object.delete"

	envSinkURI    = "SINK_URI"
	envAPIVersion = "APIVERSION_"
	envKind       = "KIND_"
	envOwnerRef   = "OWNEREF_" // format is OWNEREF_<RESOURCEINDEX>_[APIVERSION|KIND]_<OWNEREFINDEX>
)

// Add creates new ApiServerSource controllers, one per resource,
// and adds them to the manager.
func Add(mgr manager.Manager) error {
	sinkURI, defined := os.LookupEnv(envSinkURI)
	if !defined {
		return fmt.Errorf("required environment variable not defined '%s'", envSinkURI)
	}

	eventsClient, err := kncloudevents.NewDefaultClient(sinkURI)
	if err != nil {
		return err
	}

	// Create one controller per watch
	i := 0
	for {
		stri := strconv.Itoa(i)

		apiVersion, defined := os.LookupEnv(envAPIVersion + stri)
		if !defined {
			break
		}

		kind, defined := os.LookupEnv(envKind + stri)
		if !defined {
			return fmt.Errorf("required environment variable not defined '%s'", envKind+stri)
		}

		gv, err := schema.ParseGroupVersion(apiVersion)
		if err != nil {
			return err
		}

		dependents := make([]source.Kind, 0)
		j := 0
		for {
			strj := strconv.Itoa(j)

			ownerRefAPIVersion, defined := os.LookupEnv(envOwnerRef + stri + "_" + envAPIVersion + strj)
			if !defined {
				break
			}

			ownerRefKind, defined := os.LookupEnv(envOwnerRef + stri + "_" + envKind + strj)
			if !defined {
				return fmt.Errorf("required environment variable not defined '%s'", envOwnerRef+stri+"_"+envKind+strj)
			}

			ownerRefGV, err := schema.ParseGroupVersion(ownerRefAPIVersion)
			if err != nil {
				return err
			}

			u := &unstructured.Unstructured{}
			u.SetGroupVersionKind(schema.GroupVersionKind{Kind: ownerRefKind, Group: ownerRefGV.Group, Version: ownerRefGV.Version})

			dependents = append(dependents, source.Kind{Type: u})

			j++
		}

		// don't use the sdk here as we are using unstructured watches
		err = add(mgr, &reconciler{
			client:       mgr.GetClient(),
			eventsClient: eventsClient,
			gvk:          schema.GroupVersionKind{Kind: kind, Group: gv.Group, Version: gv.Version},
		}, dependents)

		if err != nil {
			return err
		}

		i++
	}
	return nil
}

// Add creates a controller for esource-controller-adapter.
func add(mgr manager.Manager, r *reconciler, dependents []source.Kind) error {
	// Setup a new controller to Reconcile Subscriptions.
	c, err := controller.New(controllerAgentName, mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch Parent events and enqueue Parent object key.
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(r.gvk)

	err = c.Watch(&source.Kind{Type: u}, &handler.EnqueueRequestForObject{})

	// Add watches for dependents
	for _, dependent := range dependents {
		err = c.Watch(&dependent, &handler.EnqueueRequestForOwner{
			IsController: true,
			OwnerType:    u,
		})

		if err != nil {
			return err
		}
	}

	return nil
}

// reconciler reconciles a ApiServerSource object
type reconciler struct {
	client       client.Client
	eventsClient eventsclient.Client
	gvk          schema.GroupVersionKind
}

func (r *reconciler) InjectClient(c client.Client) error {
	r.client = c
	return nil
}

func (r *reconciler) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	ctx := context.TODO()
	logger := logging.FromContext(ctx)

	object := &unstructured.Unstructured{}
	object.SetGroupVersionKind(r.gvk)
	err := r.client.Get(ctx, request.NamespacedName, object)

	if errors.IsNotFound(err) {
		logger.Errorf("could not find %s %v\n", request.Name, request)
		return reconcile.Result{}, nil
	}
	eventType := updateEventType
	timestamp := object.GetCreationTimestamp()
	if object.GetDeletionTimestamp() != nil {
		eventType = deleteEventType
		timestamp = *object.GetDeletionTimestamp()
	}

	objectRef := corev1.ObjectReference{
		APIVersion: object.GetAPIVersion(),
		Kind:       object.GetKind(),
		Name:       object.GetName(),
		Namespace:  object.GetNamespace(),
	}

	event := cloudevents.Event{
		Context: cloudevents.EventContextV02{
			ID:     string(object.GetUID()),
			Type:   eventType,
			Source: *types.ParseURLRef(object.GetSelfLink()),
			Time:   &types.Timestamp{Time: timestamp.Time},
		}.AsV02(),
		Data: objectRef,
	}

	if _, err := r.eventsClient.Send(ctx, event); err != nil {
		logger.Error("failed to send cloudevent (retrying)", err)

		// Retrying...
		return reconcile.Result{Requeue: true}, nil
	}

	return reconcile.Result{}, nil
}
