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

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/eventing/test/lib/resources"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
	pkgTest "knative.dev/pkg/test"

	kafkasourcev1alpha1 "knative.dev/eventing-contrib/kafka/source/pkg/apis/sources/v1alpha1"
)

func KafkaPerformanceImageSenderPod(pace string, warmup string, bootstrapUrl string, topicName string, aggregatorHostname string, additionalArgs ...string) *corev1.Pod {
	const podName = "perf-sender"
	const imageName = "kafka_performance"

	args := append([]string{
		"--roles=sender",
		fmt.Sprintf("--pace=%s", pace),
		fmt.Sprintf("--warmup=%s", warmup),
		fmt.Sprintf("--aggregator=%s:10000", aggregatorHostname),
		fmt.Sprintf("--bootstrap-url=%s", bootstrapUrl),
		fmt.Sprintf("--topic=%s", topicName),
	}, additionalArgs...)

	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: podName,
			Labels: map[string]string{
				"role": "perf-sender",
			},
		},
		Spec: corev1.PodSpec{
			ServiceAccountName: resources.PerfServiceAccount,
			RestartPolicy:      corev1.RestartPolicyNever,
			Containers: []corev1.Container{{
				Name:  "receiver",
				Image: pkgTest.ImagePath(imageName),
				Args:  args,
				Env: []corev1.EnvVar{{
					Name: "POD_NAME",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.name",
						},
					},
				}, {
					Name: "POD_NAMESPACE",
					ValueFrom: &corev1.EnvVarSource{
						FieldRef: &corev1.ObjectFieldSelector{
							FieldPath: "metadata.namespace",
						},
					},
				}},
			}},
		},
	}
}

func KafkaSource(bootstrapServer string, topicName string, ref *corev1.ObjectReference) *kafkasourcev1alpha1.KafkaSource {
	return &kafkasourcev1alpha1.KafkaSource{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-kafka-source",
		},
		Spec: kafkasourcev1alpha1.KafkaSourceSpec{
			BootstrapServers: bootstrapServer,
			Topics:           topicName,
			ConsumerGroup:    "test-consumer-group",
			Sink: &duckv1beta1.Destination{
				Ref: ref,
			},
		},
	}
}
