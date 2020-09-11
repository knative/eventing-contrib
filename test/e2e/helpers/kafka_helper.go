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
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/davecgh/go-spew/spew"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/wait"
	testlib "knative.dev/eventing/test/lib"
	pkgtest "knative.dev/pkg/test"

	sourcesv1beta1 "knative.dev/eventing-contrib/kafka/source/pkg/apis/sources/v1beta1"
	kafkaclientset "knative.dev/eventing-contrib/kafka/source/pkg/client/clientset/versioned"
)

const (
	strimziApiGroup      = "kafka.strimzi.io"
	strimziApiVersion    = "v1beta1"
	strimziTopicResource = "kafkatopics"
	interval             = 3 * time.Second
	timeout              = 30 * time.Second
)

var (
	topicGVR = schema.GroupVersionResource{Group: strimziApiGroup, Version: strimziApiVersion, Resource: strimziTopicResource}
	ImcGVR   = schema.GroupVersionResource{Group: "messaging.knative.dev", Version: "v1beta1", Resource: "inmemorychannels"}
)

func MustPublishKafkaMessage(client *testlib.Client, bootstrapServer string, topic string, key string, headers map[string]string, value string) {
	cgName := topic + "-" + key + "z"

	payload := value
	if key != "" {
		payload = key + "=" + value
	}

	cm := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cgName,
			Namespace: client.Namespace,
		},
		Data: map[string]string{
			"payload": payload,
		},
	}
	_, err := client.Kube.Kube.CoreV1().ConfigMaps(client.Namespace).Create(context.Background(), cm, metav1.CreateOptions{})
	if err != nil {
		if !apierrs.IsAlreadyExists(err) {
			client.T.Fatalf("Failed to create configmap %q: %v", cgName, err)
			return
		}
		if _, err = client.Kube.Kube.CoreV1().ConfigMaps(client.Namespace).Update(context.Background(), cm, metav1.UpdateOptions{}); err != nil {
			client.T.Fatalf("failed to update configmap: %q: %v", cgName, err)
		}
	}

	client.Tracker.Add(corev1.SchemeGroupVersion.Group, corev1.SchemeGroupVersion.Version, "configmap", client.Namespace, cgName)

	args := []string{"-P", "-T", "-b", bootstrapServer, "-t", topic}
	if key != "" {
		args = append(args, "-K=")
	}
	for k, v := range headers {
		args = append(args, "-H", k+"="+v)
	}
	args = append(args, "-l", "/etc/mounted/payload")

	client.T.Logf("Running kafkacat %s", strings.Join(args, " "))

	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      uuid.New().String() + "-producer",
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

	err = pkgtest.WaitForPodState(context.Background(), client.Kube, func(pod *corev1.Pod) (b bool, e error) {
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

func MustPublishKafkaMessageViaBinding(client *testlib.Client, selector map[string]string, topic string, key string, headers map[string]string, value string) {
	cgName := topic + "-" + key

	kvlist := make([]string, 0, len(headers))
	for k, v := range headers {
		kvlist = append(kvlist, k+":"+v)
	}

	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cgName + "-producer",
			Namespace: client.Namespace,
			Labels:    selector,
		},
		Spec: batchv1.JobSpec{
			Template: corev1.PodTemplateSpec{
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Name:  cgName + "-producer-container",
						Image: pkgtest.ImagePath("kafka-publisher"),
						Env: []corev1.EnvVar{{
							Name:  "KAFKA_TOPIC",
							Value: topic,
						}, {
							Name:  "KAFKA_KEY",
							Value: key,
						}, {
							Name:  "KAFKA_HEADERS",
							Value: strings.Join(kvlist, ","),
						}, {
							Name:  "KAFKA_VALUE",
							Value: value,
						}},
					}},
					RestartPolicy: corev1.RestartPolicyNever,
				},
			},
		},
	}

	pkgtest.CleanupOnInterrupt(func() {
		client.Kube.Kube.BatchV1().Jobs(job.Namespace).Delete(context.Background(), job.Name, metav1.DeleteOptions{})
	}, client.T.Logf)
	job, err := client.Kube.Kube.BatchV1().Jobs(job.Namespace).Create(context.Background(), job, metav1.CreateOptions{})
	if err != nil {
		client.T.Fatalf("Error creating Job: %v", err)
	}

	// Dump the state of the Job after it's been created so that we can
	// see the effects of the binding for debugging.
	client.T.Log("", "job", spew.Sprint(job))

	defer func() {
		err := client.Kube.Kube.BatchV1().Jobs(job.Namespace).Delete(context.Background(), job.Name, metav1.DeleteOptions{})
		if err != nil {
			client.T.Errorf("Error cleaning up Job %s", job.Name)
		}
	}()

	// Wait for the Job to report a successful execution.
	waitErr := wait.PollImmediate(1*time.Second, 2*time.Minute, func() (bool, error) {
		js, err := client.Kube.Kube.BatchV1().Jobs(job.Namespace).Get(context.Background(), job.Name, metav1.GetOptions{})
		if apierrs.IsNotFound(err) {
			return false, nil
		} else if err != nil {
			return true, err
		}

		client.T.Logf("Active=%d, Failed=%d, Succeeded=%d", js.Status.Active, js.Status.Failed, js.Status.Succeeded)

		// Check for successful completions.
		return js.Status.Succeeded > 0, nil
	})
	if waitErr != nil {
		client.T.Fatalf("Error waiting for Job to complete successfully: %v", waitErr)
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

	_, err := client.Dynamic.Resource(topicGVR).Namespace(clusterNamespace).Create(context.Background(), &obj, metav1.CreateOptions{})

	if err != nil {
		client.T.Fatalf("Error while creating the topic %s: %v", topicName, err)
	}

	client.Tracker.Add(topicGVR.Group, topicGVR.Version, topicGVR.Resource, clusterNamespace, topicName)
}

//CheckKafkaSourceState waits for specified kafka source resource state
//On timeout reports error
func CheckKafkaSourceState(ctx context.Context, c *testlib.Client, name string, inState func(ks *sourcesv1beta1.KafkaSource) (bool, error)) error {
	kafkaSourceClientSet, err := kafkaclientset.NewForConfig(c.Config)
	if err != nil {
		return err
	}
	kSources := kafkaSourceClientSet.SourcesV1beta1().KafkaSources(c.Namespace)
	var lastState *sourcesv1beta1.KafkaSource
	waitErr := wait.PollImmediate(interval, timeout, func() (bool, error) {
		var err error
		lastState, err = kSources.Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return true, err
		}
		return inState(lastState)
	})
	if waitErr != nil {
		return fmt.Errorf("kafkasource %q is not in desired state, got: %+v: %w", name, lastState, waitErr)
	}
	return nil
}

//CheckRADeployment waits for desired state of receiver adapter
//On timeout reports error
func CheckRADeployment(ctx context.Context, c *testlib.Client, name string, inState func(deps *appsv1.DeploymentList) (bool, error)) error {
	listOptions := metav1.ListOptions{
		LabelSelector: fmt.Sprintf("%s=%s", "eventing.knative.dev/SourceName", name),
	}
	kDeps := c.Kube.Kube.AppsV1().Deployments(c.Namespace)
	var lastState *appsv1.DeploymentList
	waitErr := wait.PollImmediate(interval, timeout, func() (bool, error) {
		var err error
		lastState, err = kDeps.List(ctx, listOptions)
		if err != nil {
			return true, err
		}
		return inState(lastState)
	})
	if waitErr != nil {
		return fmt.Errorf("receiver adapter deployments %q is not in desired state, got: %+v: %w", name, lastState, waitErr)
	}
	return nil
}
