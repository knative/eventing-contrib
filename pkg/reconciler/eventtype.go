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

package reconciler

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

const (
	// Number of characters to keep available just in case the prefix used in generateName
	// exceeds the maximum allowed for k8s names.
	generateNameSafety = 10
)

// Only allow alphanumeric, '-' or '.'.
var validChars = regexp.MustCompile(`[^-\.a-z0-9]+`)

// EventTypeReconciler is a helper struct that can be used by any source in order to reconcile its EventTypes.
type EventTypeReconciler struct {
	Client client.Client
	Scheme *runtime.Scheme
}

// EventTypeReconcilerArgs are the arguments needed to reconcile EventTypes.
type EventTypeReconcilerArgs struct {
	EventTypeSpecs []eventingv1alpha1.EventTypeSpec
	Namespace      string
	Labels         map[string]string
}

// ReconcileEventTypes reconciles the EventTypes taken from 'args', and sets 'owner' as the controller.
func (r *EventTypeReconciler) ReconcileEventTypes(ctx context.Context, owner metav1.Object, args *EventTypeReconcilerArgs) error {
	current, err := r.getEventTypes(ctx, args.Namespace, args.Labels, owner)
	if err != nil {
		return err
	}

	expected, err := r.makeEventTypes(args, owner)
	if err != nil {
		return err
	}

	toCreate := r.getEventTypesToCreate(current, expected)
	for _, eventType := range toCreate {
		err = r.Client.Create(ctx, &eventType)
		if err != nil {
			return err
		}
	}
	return nil
}

// getEventTypes returns the EventTypes controlled by 'owner'.
func (r *EventTypeReconciler) getEventTypes(ctx context.Context, namespace string, lbs map[string]string, owner metav1.Object) ([]eventingv1alpha1.EventType, error) {
	eventTypes := make([]eventingv1alpha1.EventType, 0)

	opts := &client.ListOptions{
		Namespace:     namespace,
		LabelSelector: labels.SelectorFromSet(lbs),
		// Set Raw because if we need to get more than one page, then we will put the continue token
		// into opts.Raw.Continue.
		Raw: &metav1.ListOptions{},
	}

	el := &eventingv1alpha1.EventTypeList{}
	if err := r.Client.List(ctx, opts, el); err != nil {
		return nil, err
	}

	for _, e := range el.Items {
		if metav1.IsControlledBy(&e, owner) {
			eventTypes = append(eventTypes, e)
		}
	}
	return eventTypes, nil
}

// makeEventTypes creates the in-memory representation of the EventTypes.
func (r *EventTypeReconciler) makeEventTypes(args *EventTypeReconcilerArgs, owner metav1.Object) ([]eventingv1alpha1.EventType, error) {
	eventTypes := make([]eventingv1alpha1.EventType, 0)
	for _, spec := range args.EventTypeSpecs {
		eventType := r.makeEventType(spec, args.Namespace, args.Labels)
		// Setting the reference to delete the EventType upon uninstalling the source.
		if err := controllerutil.SetControllerReference(owner, &eventType, r.Scheme); err != nil {
			return nil, err
		}
		eventTypes = append(eventTypes, eventType)
	}
	return eventTypes, nil
}

// makeEventType creates the in-memory representation of the EventType.
func (r *EventTypeReconciler) makeEventType(spec eventingv1alpha1.EventTypeSpec, namespace string, lbl map[string]string) eventingv1alpha1.EventType {
	return eventingv1alpha1.EventType{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", ToDNS1123Subdomain(spec.Type)),
			Labels:       lbl,
			Namespace:    namespace,
		},
		Spec: spec,
	}
}

// getEventTypesToCreate computes the EventTypes that need to be created based on the difference between
// 'expected' and 'current'. It does so using all the EventType.Spec fields but Description.
func (r *EventTypeReconciler) getEventTypesToCreate(current []eventingv1alpha1.EventType, expected []eventingv1alpha1.EventType) []eventingv1alpha1.EventType {
	eventTypes := make([]eventingv1alpha1.EventType, 0)
	currentMap := asMap(current, keyFromEventType)
	for _, e := range expected {
		if _, ok := currentMap[keyFromEventType(&e)]; !ok {
			// If it's not in the currentMap, we need to create it.
			eventTypes = append(eventTypes, e)
		}
	}
	return eventTypes
}

// asMap returns a map representation of 'eventTypes' list, by using the key given by 'keyFunc'.
func asMap(eventTypes []eventingv1alpha1.EventType, keyFunc func(*eventingv1alpha1.EventType) string) map[string]eventingv1alpha1.EventType {
	eventTypesAsMap := make(map[string]eventingv1alpha1.EventType, 0)
	for _, eventType := range eventTypes {
		key := keyFunc(&eventType)
		eventTypesAsMap[key] = eventType
	}
	return eventTypesAsMap
}

// Converts 'name' to a valid DNS1123 subdomain, required for object names in K8s.
func ToDNS1123Subdomain(name string) string {
	// If it is not a valid DNS1123 subdomain, make it a valid one.
	if msgs := validation.IsDNS1123Subdomain(name); len(msgs) != 0 {
		// If the length exceeds the max, cut it and leave some room for a potential generated UUID.
		if len(name) > validation.DNS1123SubdomainMaxLength {
			name = name[:validation.DNS1123SubdomainMaxLength-generateNameSafety]
		}
		name = strings.ToLower(name)
		name = validChars.ReplaceAllString(name, "")
		// Only start/end with alphanumeric.
		name = strings.Trim(name, "-.")
	}
	return name
}

func keyFromEventType(eventType *eventingv1alpha1.EventType) string {
	return fmt.Sprintf("%s_%s_%s_%s", eventType.Spec.Type, eventType.Spec.Source, eventType.Spec.Schema, eventType.Spec.Broker)
}
