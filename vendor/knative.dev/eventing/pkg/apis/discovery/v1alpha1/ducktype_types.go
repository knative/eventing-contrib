/*
Copyright 2020 The Knative Authors.

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

package v1alpha1

import (
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/apis"
	duckv1 "knative.dev/pkg/apis/duck/v1"
	"knative.dev/pkg/kmeta"
)

// +genclient
// +genclient:nonNamespaced
// +genreconciler
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DuckType is a Knative abstraction for kubernetes resource duck type discovery.
type DuckType struct {
	metav1.TypeMeta `json:",inline"`
	// +optional
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec holds the desired state of the DuckType (from the client).
	// +optional
	Spec DuckTypeSpec `json:"spec,omitempty"`

	// Status communicates the observed state of the DuckType (from the controller).
	// +optional
	Status DuckTypeStatus `json:"status,omitempty"`
}

// Check that DuckType can be validated and defaulted.
var _ apis.Validatable = (*DuckType)(nil)
var _ apis.Defaultable = (*DuckType)(nil)
var _ kmeta.OwnerRefable = (*DuckType)(nil)

// DuckTypeSpec holds the desired state of the DuckType (from the client).
type DuckTypeSpec struct {
	// Discovery

	// SelectorType
	SelectorType []CustomResourceDefinitionType `json:"selectors,omitempty"`

	// Manual Discovery

	// RefsList
	RefsList []GroupVersionResourceKind `json:"refs,omitempty"`

	// DuckType names and info
	Names NamesSpec `json:"names"`

	// Custom Columns
	AdditionalPrinterColumns []apiextensionsv1beta1.CustomResourceColumnDefinition `json:"additionalPrinterColumns,omitempty"`

	// Partial Schema
	Schema *apiextensionsv1beta1.CustomResourceValidation `json:"schema,omitempty"`
}

const (
	// DuckTypeConditionReady is set when the revision is starting to materialize
	// runtime resources, and becomes true when those resources are ready.
	DuckTypeConditionReady = apis.ConditionReady
)

// CustomResourceDefinitionType
type CustomResourceDefinitionType struct {
	// Query is the fully filled label selected.
	Selector string `json:"selector,omitempty"`
}

// NamesSpec
type NamesSpec struct {
	Plural   string `json:"plural,omitempty"`
	Singular string `json:"singular,omitempty"`
}

// DuckTypeStatus communicates the observed state of the DuckType (from the controller).
type DuckTypeStatus struct {
	duckv1.Status `json:",inline"`

	// DuckList
	DuckList []GroupVersionResourceKind `json:"ducks,omitempty"`

	// DuckCount
	DuckCount int `json:"duckCount,omitempty"`
}

// GroupVersionResourceKind
type GroupVersionResourceKind struct {
	Group    string `json:"group,omitempty"`
	Version  string `json:"version,omitempty"`
	Resource string `json:"resource,omitempty"`
	Kind     string `json:"kind,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DuckTypeList is a list of DuckType resources
type DuckTypeList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []DuckType `json:"items"`
}
