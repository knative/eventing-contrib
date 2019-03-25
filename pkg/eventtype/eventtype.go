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

	"k8s.io/apimachinery/pkg/runtime"

	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/sets"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

type EventTypeReconciler struct {
	Client client.Client
	Scheme *runtime.Scheme
}

// EventTypeArgs are the arguments needed to create an EventType.
type EventTypesArgs struct {
	Args      []*EventTypeArgs
	Namespace string
	Labels    map[string]string
}

// EventTypeArgs are the arguments needed to create an EventType.
type EventTypeArgs struct {
	Type   string
	Source string
	Schema string
	Broker string
}

func (r *EventTypeReconciler) ReconcileEventTypes(ctx context.Context, owner metav1.Object, args *EventTypesArgs) error {
	current, err := r.getEventTypes(ctx, args.Namespace, args.Labels, owner)
	if err != nil {
		return err
	}

	expected, err := r.newEventTypes(args, owner)
	if err != nil {
		return err
	}

	diff := r.difference(current, expected)
	if len(diff) > 0 {
		// As EventTypes are immutable, if we have any diff it means that something was deleted or not even created
		// in the first place. We should create them.
		for _, eventType := range diff {
			err = r.Client.Create(ctx, eventType)
			if err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *EventTypeReconciler) getEventTypes(ctx context.Context, namespace string, lbs map[string]string, owner metav1.Object) ([]*eventingv1alpha1.EventType, error) {
	eventTypes := make([]*eventingv1alpha1.EventType, 0)

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
				eventTypes = append(eventTypes, &e)
			}
		}
		if el.Continue != "" {
			opts.Raw.Continue = el.Continue
		} else {
			return eventTypes, nil
		}
	}
}

func (r *EventTypeReconciler) newEventTypes(args *EventTypesArgs, owner metav1.Object) ([]*eventingv1alpha1.EventType, error) {
	eventTypes := make([]*eventingv1alpha1.EventType, 0)
	for _, arg := range args.Args {
		eventType := r.makeEventType(arg, args.Namespace, args.Labels)
		// Setting the reference to delete the EventType upon uninstallation of the source.
		if err := controllerutil.SetControllerReference(owner, eventType, r.Scheme); err != nil {
			return nil, err
		}
		eventTypes = append(eventTypes, eventType)
	}
	return eventTypes, nil
}

func (r *EventTypeReconciler) makeEventType(args *EventTypeArgs, namespace string, lbl map[string]string) *eventingv1alpha1.EventType {
	return &eventingv1alpha1.EventType{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", utils.ToDNS1123Subdomain(args.Type)),
			Labels:       lbl,
			Namespace:    namespace,
		},
		Spec: eventingv1alpha1.EventTypeSpec{
			Type:   args.Type,
			Source: args.Source,
			Schema: args.Schema,
			Broker: args.Broker,
		},
	}
}

// Computes the difference between 'current' and 'expected' based on the EventType.Spec fields. As EventTypes are
// immutable, a difference means that something got deleted or not created.
func (r *EventTypeReconciler) difference(current []*eventingv1alpha1.EventType, expected []*eventingv1alpha1.EventType) []*eventingv1alpha1.EventType {
	difference := make([]*eventingv1alpha1.EventType, 0)
	nameFunc := func(eventType *eventingv1alpha1.EventType) string {
		return fmt.Sprintf("%s_%s_%s_%s", eventType.Spec.Type, eventType.Spec.Source, eventType.Spec.Schema, eventType.Spec.Broker)
	}
	currentSet := asSet(current, nameFunc)
	for _, e := range expected {
		if !currentSet.Has(nameFunc(e)) {
			difference = append(difference, e)
		}
	}
	return difference
}

func asSet(eventTypes []*eventingv1alpha1.EventType, nameFunc func(*eventingv1alpha1.EventType) string) *sets.String {
	set := &sets.String{}
	for _, eventType := range eventTypes {
		set.Insert(nameFunc(eventType))
	}
	return set
}
