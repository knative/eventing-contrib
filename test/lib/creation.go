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

package lib

import (
	"knative.dev/eventing/test/lib"

	bindingsv1alpha1 "knative.dev/eventing-contrib/kafka/source/pkg/apis/bindings/v1alpha1"
	sourcesv1alpha1 "knative.dev/eventing-contrib/kafka/source/pkg/apis/sources/v1alpha1"
	kafkaclientset "knative.dev/eventing-contrib/kafka/source/pkg/client/clientset/versioned"
)

func CreateKafkaSourceOrFail(c *lib.Client, kafkaSource *sourcesv1alpha1.KafkaSource) {
	kafkaSourceClientSet, err := kafkaclientset.NewForConfig(c.Config)
	if err != nil {
		c.T.Fatalf("Failed to create KafkaSource client: %v", err)
	}

	kSources := kafkaSourceClientSet.SourcesV1alpha1().KafkaSources(c.Namespace)
	if createdKafkaSource, err := kSources.Create(kafkaSource); err != nil {
		c.T.Fatalf("Failed to create KafkaSource %q: %v", kafkaSource.Name, err)
	} else {
		c.Tracker.AddObj(createdKafkaSource)
	}
}

func CreateKafkaBindingOrFail(c *lib.Client, kafkaBinding *bindingsv1alpha1.KafkaBinding) {
	kafkaBindingClientSet, err := kafkaclientset.NewForConfig(c.Config)
	if err != nil {
		c.T.Fatalf("Failed to create KafkaBinding client: %v", err)
	}

	kBindings := kafkaBindingClientSet.BindingsV1alpha1().KafkaBindings(c.Namespace)
	if createdKafkaBinding, err := kBindings.Create(kafkaBinding); err != nil {
		c.T.Fatalf("Failed to create KafkaBinding %q: %v", kafkaBinding.Name, err)
	} else {
		c.Tracker.AddObj(createdKafkaBinding)
	}
}
