/*
Copyright 2020 The Knative Authors

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
	camelv1 "github.com/apache/camel-k/pkg/apis/camel/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakekubeclientset "k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/cache"
	fakeeventingclientset "knative.dev/eventing/pkg/client/clientset/versioned/fake"
	"knative.dev/pkg/reconciler/testing"

	fakecamelclientset "github.com/apache/camel-k/pkg/client/clientset/versioned/fake"
	camellisters "github.com/apache/camel-k/pkg/client/listers/camel/v1"
	sourcesv1alpha1 "knative.dev/eventing-contrib/camel/source/pkg/apis/sources/v1alpha1"
	fakesourcesclientset "knative.dev/eventing-contrib/camel/source/pkg/client/clientset/versioned/fake"
	sourceslisters "knative.dev/eventing-contrib/camel/source/pkg/client/listers/sources/v1alpha1"
)

var addressableAddToScheme = func(scheme *runtime.Scheme) error {
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{Group: "duck.knative.dev", Version: "v1", Kind: "Addressable"}, &unstructured.Unstructured{})
	return nil
}

var clientSetSchemes = []func(*runtime.Scheme) error{
	fakekubeclientset.AddToScheme,
	fakesourcesclientset.AddToScheme,
	fakecamelclientset.AddToScheme,
	fakeeventingclientset.AddToScheme,
	addressableAddToScheme,
}

type Listers struct {
	sorter testing.ObjectSorter
}

func NewListers(objs []runtime.Object) Listers {
	scheme := runtime.NewScheme()

	for _, addTo := range clientSetSchemes {
		addTo(scheme)
	}

	ls := Listers{
		sorter: testing.NewObjectSorter(scheme),
	}

	ls.sorter.AddObjects(objs...)

	return ls
}

func (l *Listers) indexerFor(obj runtime.Object) cache.Indexer {
	return l.sorter.IndexerForObjectType(obj)
}

func (l *Listers) GetCamelkObjects() []runtime.Object {
	return l.sorter.ObjectsForSchemeFunc(fakecamelclientset.AddToScheme)
}

func (l *Listers) GetSourceObjects() []runtime.Object {
	return l.sorter.ObjectsForSchemeFunc(fakesourcesclientset.AddToScheme)
}

func (l *Listers) GetAddressableObjects() []runtime.Object {
	return l.sorter.ObjectsForSchemeFunc(addressableAddToScheme)
}

func (l *Listers) GetAllObjects() []runtime.Object {
	all := l.GetSourceObjects()
	all = append(all, l.GetAddressableObjects()...)
	all = append(all, l.GetCamelkObjects()...)
	return all
}

func (l *Listers) GetCamelSourcesLister() sourceslisters.CamelSourceLister {
	return sourceslisters.NewCamelSourceLister(l.indexerFor(&sourcesv1alpha1.CamelSource{}))
}

func (l *Listers) GetCamelkIntegrationLister() camellisters.IntegrationLister {
	return camellisters.NewIntegrationLister(l.indexerFor(&camelv1.Integration{}))
}
