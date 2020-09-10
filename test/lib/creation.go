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
	"context"

	testlib "knative.dev/eventing/test/lib"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	bindingsv1alpha1 "knative.dev/eventing-contrib/kafka/source/pkg/apis/bindings/v1alpha1"
	bindingsv1beta1 "knative.dev/eventing-contrib/kafka/source/pkg/apis/bindings/v1beta1"
	sourcesv1alpha1 "knative.dev/eventing-contrib/kafka/source/pkg/apis/sources/v1alpha1"
	sourcesv1beta1 "knative.dev/eventing-contrib/kafka/source/pkg/apis/sources/v1beta1"
	kafkaclientset "knative.dev/eventing-contrib/kafka/source/pkg/client/clientset/versioned"
)

func CreateKafkaSourceV1Alpha1OrFail(c *testlib.Client, kafkaSource *sourcesv1alpha1.KafkaSource) {
	kafkaSourceClientSet, err := kafkaclientset.NewForConfig(c.Config)
	if err != nil {
		c.T.Fatalf("Failed to create v1alpha1 KafkaSource client: %v", err)
	}

	kSources := kafkaSourceClientSet.SourcesV1alpha1().KafkaSources(c.Namespace)
	if createdKafkaSource, err := kSources.Create(context.Background(), kafkaSource, metav1.CreateOptions{}); err != nil {
		c.T.Fatalf("Failed to create v1alpha1 KafkaSource %q: %v", kafkaSource.Name, err)
	} else {
		c.Tracker.AddObj(createdKafkaSource)
	}
}

func CreateKafkaSourceV1Beta1OrFail(c *testlib.Client, kafkaSource *sourcesv1beta1.KafkaSource) {
	kafkaSourceClientSet, err := kafkaclientset.NewForConfig(c.Config)
	if err != nil {
		c.T.Fatalf("Failed to create v1beta1 KafkaSource client: %v", err)
	}

	kSources := kafkaSourceClientSet.SourcesV1beta1().KafkaSources(c.Namespace)
	if createdKafkaSource, err := kSources.Create(context.Background(), kafkaSource, metav1.CreateOptions{}); err != nil {
		c.T.Fatalf("Failed to create v1beta1 KafkaSource %q: %v", kafkaSource.Name, err)
	} else {
		c.Tracker.AddObj(createdKafkaSource)
	}
}

func CreateKafkaBindingV1Alpha1OrFail(c *testlib.Client, kafkaBinding *bindingsv1alpha1.KafkaBinding) {
	kafkaBindingClientSet, err := kafkaclientset.NewForConfig(c.Config)
	if err != nil {
		c.T.Fatalf("Failed to create v1alpha1 KafkaBinding client: %v", err)
	}

	kBindings := kafkaBindingClientSet.BindingsV1alpha1().KafkaBindings(c.Namespace)
	if createdKafkaBinding, err := kBindings.Create(context.Background(), kafkaBinding, metav1.CreateOptions{}); err != nil {
		c.T.Fatalf("Failed to create v1alpha1 KafkaBinding %q: %v", kafkaBinding.Name, err)
	} else {
		c.Tracker.AddObj(createdKafkaBinding)
	}
}

func CreateKafkaBindingV1Beta1OrFail(c *testlib.Client, kafkaBinding *bindingsv1beta1.KafkaBinding) {
	kafkaBindingClientSet, err := kafkaclientset.NewForConfig(c.Config)
	if err != nil {
		c.T.Fatalf("Failed to create v1beta1 KafkaBinding client: %v", err)
	}

	kBindings := kafkaBindingClientSet.BindingsV1beta1().KafkaBindings(c.Namespace)
	if createdKafkaBinding, err := kBindings.Create(context.Background(), kafkaBinding, metav1.CreateOptions{}); err != nil {
		c.T.Fatalf("Failed to create v1beta1 KafkaBinding %q: %v", kafkaBinding.Name, err)
	} else {
		c.Tracker.AddObj(createdKafkaBinding)
	}
}
