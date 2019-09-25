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

package testing

import (
	//	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"knative.dev/eventing-contrib/kafka/source/pkg/apis/sources/v1alpha1"
)

type KafkaSourceOption func(*v1alpha1.KafkaSource)

//We create a new KafkaSource based on the options passed to it.
func NewKafkaSource(name, namespace string, options ...KafkaSourceOption) *v1alpha1.KafkaSource {
	k := &v1alpha1.KafkaSource{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
	}
	for _, opt := range options {
		opt(k)
	}
	return k
}

func WithKafkaSourceUID(uid string) KafkaSourceOption {
	return func(src *v1alpha1.KafkaSource) {
		src.UID = types.UID(uid)
	}
}

// WithInitKafkaSourceConditions initializes the KafkaSource's conditions.
func WithInitKafkaSourceConditions(src *v1alpha1.KafkaSource) {
	src.Status.InitializeConditions()
}

func WithKafkaSourceSpec(spec v1alpha1.KafkaSourceSpec) KafkaSourceOption {
	return func(src *v1alpha1.KafkaSource) {
		src.Spec = spec
	}
}

func WithKafkaSourceObjectMetaGeneration(generation int64) KafkaSourceOption {
	return func(src *v1alpha1.KafkaSource) {
		src.ObjectMeta.Generation = generation
	}
}

func WithKafkaSourceStatusObservedGeneration(generation int64) KafkaSourceOption {
	return func(src *v1alpha1.KafkaSource) {
		src.Status.ObservedGeneration = generation
	}
}

func WithKafkaSourceSinkNotFound(src *v1alpha1.KafkaSource) {
	src.Status.MarkNoSink("NotFound", "")
}
