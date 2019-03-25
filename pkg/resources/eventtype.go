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

package resources

import (
	"fmt"

	eventingv1alpha1 "github.com/knative/eventing/pkg/apis/eventing/v1alpha1"
	"github.com/knative/eventing/pkg/utils"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

// EventTypeArgs are the arguments needed to create an EventType.
type EventTypeArgs struct {
	Type      string
	Source    string
	Schema    string
	Broker    string
	Labels    map[string]string
	Namespace string
}

func MakeEventType(args *EventTypeArgs) *eventingv1alpha1.EventType {
	return &eventingv1alpha1.EventType{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: fmt.Sprintf("%s-", utils.ToDNS1123Subdomain(args.Type)),
			Labels:       args.Labels,
			Namespace:    args.Namespace,
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
func Difference(current []*eventingv1alpha1.EventType, expected []*eventingv1alpha1.EventType) []*eventingv1alpha1.EventType {
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

func asSet(eventTypes []*eventingv1alpha1.EventType, nameFunc func(*eventingv1alpha1.EventType) string) sets.String {
	set := sets.String{}
	for _, eventType := range eventTypes {
		set.Insert(nameFunc(eventType))
	}
	return set
}
