/*
Copyright 2018 The Knative Authors

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

package sdk

import (
	"reflect"
)

// StatusAccessor is the interface for a Resource that implements the getter and
// setter for accessing a Condition collection.
// +k8s:deepcopy-gen=true
type StatusAccessor interface {
	GetStatus() interface{}
	SetStatus(interface{})
}

// NewReflectedStatusAccessor uses reflection to return a StatusAccessor
// to access the field called "Status".
func NewReflectedStatusAccessor(object interface{}) StatusAccessor {
	objectValue := reflect.Indirect(reflect.ValueOf(object))

	// If object is not a struct, don't even try to use it.
	if objectValue.Kind() != reflect.Struct {
		return nil
	}

	statusField := objectValue.FieldByName("Status")

	if statusField.IsValid() && statusField.CanInterface() && statusField.CanSet() {
		if _, ok := statusField.Interface().(interface{}); ok {
			return &reflectedStatusAccessor{
				status: statusField,
			}
		}
	}
	return nil
}

// reflectedConditionsAccessor is an internal wrapper object to act as the
// ConditionsAccessor for status objects that do not implement ConditionsAccessor
// directly, but do expose the field using the "Conditions" field name.
type reflectedStatusAccessor struct {
	status reflect.Value
}

// GetConditions uses reflection to return Conditions from the held status object.
func (r *reflectedStatusAccessor) GetStatus() interface{} {
	if r != nil && r.status.IsValid() && r.status.CanInterface() {
		if status, ok := r.status.Interface().(interface{}); ok {
			return status
		}
	}
	return nil
}

// SetConditions uses reflection to set Conditions on the held status object.
func (r *reflectedStatusAccessor) SetStatus(status interface{}) {
	if r != nil && r.status.IsValid() && r.status.CanSet() {
		r.status.Set(reflect.ValueOf(status))
	}
}

// FinalizersAccessor is the interface for a Resource that implements the getter and setting for
// accessing its Finalizer set.
// +k8s:deepcopy-gen=true
type FinalizersAccessor interface {
	GetFinalizers() interface{}
	SetFinalizers(finalizers interface{})
}

// NewReflectedFinalizersAccessor uses reflection to return a FinalizersAccessor to access the field
// called "Finalizers".
func NewReflectedFinalizersAccessor(object interface{}) FinalizersAccessor {
	objectValue := reflect.Indirect(reflect.ValueOf(object))

	// If object is not a struct, don't even try to use it.
	if objectValue.Kind() != reflect.Struct {
		return nil
	}

	finalizerField := objectValue.FieldByName("Finalizers")
	if finalizerField.IsValid() && finalizerField.CanInterface() && finalizerField.CanSet() {
		if _, ok := finalizerField.Interface().(interface{}); ok {
			return &reflectedFinalizersAccessor{
				finalizers: finalizerField,
			}
		}
	}
	return nil
}

// reflectedFinalizersAccessor is an internal wrapper object to act as the FinalizersAccessor for
// objects that do not implement FinalizersAccessor directly, but do expose the field using the
// name "Finalizers".
type reflectedFinalizersAccessor struct {
	finalizers reflect.Value
}

// GetFinalizers uses reflection to return the Finalizers set from the held object.
func (r *reflectedFinalizersAccessor) GetFinalizers() interface{} {
	if r != nil && r.finalizers.IsValid() && r.finalizers.CanInterface() {
		if finalizers, ok := r.finalizers.Interface().(interface{}); ok {
			return finalizers
		}
	}
	return nil
}

// SetFinalizers uses reflection to set Finalizers on the held object.
func (r *reflectedFinalizersAccessor) SetFinalizers(finalizers interface{}) {
	if r != nil && r.finalizers.IsValid() && r.finalizers.CanSet() {
		r.finalizers.Set(reflect.ValueOf(finalizers))
	}
}
