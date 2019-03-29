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

package eventtype

import (
	"context"
	"fmt"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/utils"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

// Reconciler reconciles EventTypes of different sources.
type Reconciler struct {
	Client client.Client
	Scheme *runtime.Scheme
}

// ReconcilerArgs are the arguments needed to reconcile EventTypes.
type ReconcilerArgs struct {
	EventTypes []*EventTypeArgs
	Namespace  string
	Labels     map[string]string
}

// EventTypeArgs are the arguments needed to create an EventType.
type EventTypeArgs struct {
	Type   string
	Source string
	Schema string
	Broker string
}

// syncEventTypes is a helper struct used to sync EventTypes during the reconciliation process.
type syncEventTypes struct {
	ToCreate []eventingv1alpha1.EventType
	ToDelete []eventingv1alpha1.EventType
}

// ReconcileEventTypes reconciles the EventTypes taken from 'args', and sets 'owner' as the controller.
func (r *Reconciler) ReconcileEventTypes(ctx context.Context, owner metav1.Object, args *ReconcilerArgs) error {
	current, err := r.getEventTypes(ctx, args.Namespace, args.Labels, owner)
	if err != nil {
		return err
	}

	expected, err := r.makeEventTypes(args, owner)
	if err != nil {
		return err
	}
	sync := r.getEventTypesToSync(current, expected)
	for _, eventType := range sync.ToCreate {
		err = r.Client.Create(ctx, &eventType)
		if err != nil {
			return err
		}
	}
	// Deletion might not be needed, but just to make sure we dedupe EventTypes
	// in case some source controller has a bug.
	for _, eventType := range sync.ToDelete {
		err = r.Client.Delete(ctx, &eventType)
		if err != nil && !errors.IsNotFound(err) {
			return err
		}
	}
	return nil
}

// getEventTypes returns the EventTypes controlled by 'owner'.
func (r *Reconciler) getEventTypes(ctx context.Context, namespace string, lbs map[string]string, owner metav1.Object) ([]eventingv1alpha1.EventType, error) {
	eventTypes := make([]eventingv1alpha1.EventType, 0)

	opts := &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: labels.SelectorFromSet(lbs),
		// Set Raw because if we need to get more than one page, then we will put the continue token
		// into opts.Raw.Continue.
		Raw: &metav1.ListOptions{},
	}
	for {
		el := &eventingv1alpha1.EventTypeList{}
		if err := r.Client.List(ctx, opts, el); err != nil {
			return nil, err
		}

		for _, e := range el.Items {
			if metav1.IsControlledBy(&e, owner) {
				eventTypes = append(eventTypes, e)
			}
		}
		if el.Continue != "" {
			opts.Raw.Continue = el.Continue
		} else {
			return eventTypes, nil
		}
	}
}

// makeEventTypes creates the in-memory representation of the EventTypes.
func (r *Reconciler) makeEventTypes(args *ReconcilerArgs, owner metav1.Object) ([]eventingv1alpha1.EventType, error) {
	eventTypes := make([]eventingv1alpha1.EventType, 0)
	for _, arg := range args.EventTypes {
		eventType := r.makeEventType(arg, args.Namespace, args.Labels)
		// Setting the reference to delete the EventType upon uninstalling the source.
		if err := controllerutil.SetControllerReference(owner, &eventType, r.Scheme); err != nil {
			return nil, err
		}
		eventTypes = append(eventTypes, eventType)
	}
	return eventTypes, nil
}

// makeEventType creates the in-memory representation of the EventType.
func (r *Reconciler) makeEventType(arg *EventTypeArgs, namespace string, lbl map[string]string) eventingv1alpha1.EventType {
	return eventingv1alpha1.EventType{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", utils.ToDNS1123Subdomain(arg.Type)),
			Labels:       lbl,
			Namespace:    namespace,
		},
		Spec: eventingv1alpha1.EventTypeSpec{
			Type:   arg.Type,
			Source: arg.Source,
			Schema: arg.Schema,
			Broker: arg.Broker,
		},
	}
}

// getSyncEventTypes computes the EventTypes that need to be created and/or deleted based on the difference between
// 'expected' and 'current'. It does so using all the EventType.Spec fields.
func (r *Reconciler) getEventTypesToSync(current []eventingv1alpha1.EventType, expected []eventingv1alpha1.EventType) *syncEventTypes {
	eventTypesToSync := &syncEventTypes{
		ToCreate: make([]eventingv1alpha1.EventType, 0),
		ToDelete: make([]eventingv1alpha1.EventType, 0),
	}
	keyFunc := func(eventType *eventingv1alpha1.EventType) string {
		return fmt.Sprintf("%s_%s_%s_%s", eventType.Spec.Type, eventType.Spec.Source, eventType.Spec.Schema, eventType.Spec.Broker)
	}

	currentMap := groupByKey(current, keyFunc)
	for _, e := range expected {
		if et, ok := currentMap[keyFunc(&e)]; !ok {
			// If it's not in the currentMap, we need to create it.
			eventTypesToSync.ToCreate = append(eventTypesToSync.ToCreate, e)
		} else {
			// If it's in the currentMap and there are more than one,
			// then we just keep the first one, and delete the others.
			if len(et) > 1 {
				for i := 1; i < len(et); i++ {
					eventTypesToSync.ToDelete = append(eventTypesToSync.ToDelete, et[i])
				}
			}
		}
	}
	return eventTypesToSync
}

// groupByKey groups 'eventTypes' by key, where the key is given by 'keyFunc'.
func groupByKey(eventTypes []eventingv1alpha1.EventType, keyFunc func(*eventingv1alpha1.EventType) string) map[string][]eventingv1alpha1.EventType {
	eventTypesAsMap := make(map[string][]eventingv1alpha1.EventType, 0)
	for _, eventType := range eventTypes {
		key := keyFunc(&eventType)
		eventTypesAsMap[key] = append(eventTypesAsMap[key], eventType)
	}
	return eventTypesAsMap
}
