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

package helpers

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	testlib "knative.dev/eventing/test/lib"
	pkgtest "knative.dev/pkg/test"
)

const (
	strimziApiGroup      = "kafka.strimzi.io"
	strimziApiVersion    = "v1beta1"
	strimziTopicResource = "kafkatopics"
)

var (
	topicGVR = schema.GroupVersionResource{Group: strimziApiGroup, Version: strimziApiVersion, Resource: strimziTopicResource}
)

func MustPublishKafkaMessage(client *testlib.Client, bootstrapServer string, topic string, key string, headers map[string]string, value string) {
	cgName := topic + "-" + key

	_, err := client.Kube.Kube.CoreV1().ConfigMaps(client.Namespace).Create(&corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      topic + "-" + key,
			Namespace: client.Namespace,
		},
		Data: map[string]string{
			"payload": value,
		},
	})
	if err != nil {
		client.T.Fatalf("Failed to create configmap %q: %v", cgName, err)
		return
	}

	client.Tracker.Add(corev1.SchemeGroupVersion.Group, corev1.SchemeGroupVersion.Version, "configmap", client.Namespace, cgName)

	var args []string
	args = append(args, "-P", "-v", "-b", bootstrapServer, "-t", topic, "-k", key)
	for k, v := range headers {
		args = append(args, "-H", k+"="+v)
	}
	args = append(args, "/etc/mounted/payload")

	client.T.Logf("Running kafkacat %v", args)

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cgName + "-producer",
			Namespace: client.Namespace,
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{{
				Image:   "docker.io/edenhill/kafkacat:1.5.0",
				Name:    cgName + "-producer-container",
				Command: []string{"kafkacat"},
				Args:    args,
				VolumeMounts: []corev1.VolumeMount{{
					Name:      "event-payload",
					MountPath: "/etc/mounted",
				}},
			}},
			RestartPolicy: corev1.RestartPolicyNever,
			Volumes: []corev1.Volume{{
				Name: "event-payload",
				VolumeSource: corev1.VolumeSource{ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cgName,
					},
				}},
			}},
		},
	}
	client.CreatePodOrFail(&pod)

	err = pkgtest.WaitForPodState(client.Kube, func(pod *corev1.Pod) (b bool, e error) {
		if pod.Status.Phase == corev1.PodFailed {
			return true, fmt.Errorf("aggregator pod failed with message %s", pod.Status.Message)
		} else if pod.Status.Phase != corev1.PodSucceeded {
			return false, nil
		}
		return true, nil
	}, pod.Name, pod.Namespace)
	if err != nil {
		client.T.Fatalf("Failed waiting for pod for completeness %q: %v", pod.Name, err)
	}
}

func MustCreateTopic(client *testlib.Client, clusterName string, clusterNamespace string, topicName string) {
	obj := unstructured.Unstructured{
		Object: map[string]interface{}{
			"apiVersion": topicGVR.GroupVersion().String(),
			"kind":       "KafkaTopic",
			"metadata": map[string]interface{}{
				"name": topicName,
				"labels": map[string]interface{}{
					"strimzi.io/cluster": clusterName,
				},
			},
			"spec": map[string]interface{}{
				"partitions": 10,
				"replicas":   1,
			},
		},
	}

	_, err := client.Dynamic.Resource(topicGVR).Namespace(clusterNamespace).Create(&obj, metav1.CreateOptions{})

	if err != nil {
		client.T.Fatalf("Error while creating the topic %s: %v", topicName, err)
	}

	client.Tracker.Add(topicGVR.Group, topicGVR.Version, topicGVR.Resource, clusterNamespace, topicName)
}
